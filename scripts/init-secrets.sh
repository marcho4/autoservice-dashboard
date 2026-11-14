#!/usr/bin/env bash
# Один раз на новый кластер: создаёт секрет app-secret со случайными паролем БД и ключом сессий.
# Существующий секрет не трогает: пароль БД задаётся при первом запуске PostgreSQL и потом не меняется.
#
#   LLM_API_KEY=<ключ LLM> ./scripts/init-secrets.sh
#
# Поменять ключ LLM позже:
#   kubectl -n autoservice patch secret app-secret -p '{"stringData":{"LLM_API_KEY":"<ключ>"}}'
#   kubectl -n autoservice rollout restart deployment/backend
set -euo pipefail
cd "$(dirname "$0")/../deploy"

NS=autoservice
kubectl apply -f 00-namespace.yml
if kubectl -n "$NS" get secret app-secret >/dev/null 2>&1; then
  echo "init-secrets: секрет app-secret уже есть, ничего не меняю" >&2
  exit 1
fi

# BOT_API_KEY пустой: internal API выключен, встроенному боту он не нужен.
kubectl -n "$NS" create secret generic app-secret \
  --from-literal=POSTGRES_USER=autoservice \
  --from-literal=POSTGRES_PASSWORD="$(openssl rand -hex 24)" \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=BOT_API_KEY= \
  --from-literal=LLM_API_KEY="${LLM_API_KEY:-}"
