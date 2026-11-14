#!/usr/bin/env bash
# Выкатка версии в кластер, на который смотрит текущий контекст kubectl.
# Образы этой версии уже должны лежать в GHCR (их собирает CI по тегу, см. scripts/release.sh).
# Откат — тот же скрипт с предыдущей версией.
#
#   ./scripts/deploy.sh v1.2.3
#   YES=1 ./scripts/deploy.sh v1.2.3   # без подтверждения
set -euo pipefail
cd "$(dirname "$0")/../deploy"

NS=autoservice
die() { echo "deploy: $*" >&2; exit 1; }

version=${1:-}
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "укажите версию вида v1.2.3"

context=$(kubectl config current-context)
previous=$(kubectl -n "$NS" get deployment backend \
  -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null | sed 's/.*://' || true)

echo "Кластер: $context"
echo "Версия:  ${previous:-нет} -> $version"
if [[ ${YES:-} != 1 ]]; then
  read -r -p "Выкатываем? [y/N] " answer
  [[ $answer == [yY] ]] || die "отменено"
fi

kubectl apply -f 00-namespace.yml
kubectl -n "$NS" get secret app-secret >/dev/null 2>&1 \
  || die "нет секрета app-secret: один раз запустите ./scripts/init-secrets.sh"

kubectl apply -f config-map.yml -f alertConfigMap.yml -f services.yml
# В манифестах вместо тега стоит VERSION. Миграции идут init-контейнером бэкенда из того же образа.
sed -E "s#(ghcr\.io/[^:]+/autoservice-(backend|frontend)):VERSION#\1:$version#" deployments.yml \
  | kubectl apply -f -

for name in backend frontend prober; do
  if ! kubectl -n "$NS" rollout status "deployment/$name" --timeout=5m; then
    kubectl -n "$NS" get pods >&2
    echo "deploy: $name не поднялся на $version; прежние поды продолжают обслуживать запросы." >&2
    echo "deploy: причина: kubectl -n $NS describe pods -l app=$name; kubectl -n $NS logs -l app=$name --all-containers" >&2
    [[ $previous =~ ^v[0-9] ]] && echo "deploy: откат: YES=1 ./scripts/deploy.sh $previous" >&2
    exit 1
  fi
done
echo "Готово: $version выкачена в $context."
