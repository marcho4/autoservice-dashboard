#!/usr/bin/env bash
# Мониторинг в кластере: VictoriaMetrics, vmalert + Alertmanager, VictoriaLogs, Grafana.
# Сначала приложение: ./scripts/deploy.sh vX.Y.Z (namespace autoservice и ConfigMap alerts).
# Первый запуск: TELEGRAM_BOT_TOKEN=<токен бота для алертов> ./deploy/monitoring/install.sh
# Повторный запуск обновляет релизы под текущие values, токен тогда можно не передавать.
set -euo pipefail
cd "$(dirname "$0")"

helm repo add vm https://victoriametrics.github.io/helm-charts/ --force-update
helm repo add grafana-community https://grafana-community.github.io/helm-charts --force-update
helm repo update

# Метрики
helm upgrade --install vmsingle vm/victoria-metrics-single --version 0.49.0 \
  -n autoservice -f vmsingle-values.yaml

# Алерты. Токен бота держим в секрете, а не в values: репозиторий публичный.
if [[ -n "${TELEGRAM_BOT_TOKEN:-}" ]]; then
  kubectl -n autoservice create secret generic alertmanager-telegram \
    --from-literal=bot_token="$TELEGRAM_BOT_TOKEN" --dry-run=client -o yaml | kubectl apply -f -
fi
kubectl -n autoservice get secret alertmanager-telegram >/dev/null || {
  echo "Нет секрета alertmanager-telegram: запустите с TELEGRAM_BOT_TOKEN=<токен>" >&2
  exit 1
}
helm upgrade --install vmalert vm/victoria-metrics-alert --version 0.52.0 \
  -n autoservice -f vmalert-values.yaml

# Логи
helm upgrade --install vlogs vm/victoria-logs-single --version 0.13.10 \
  -n logging --create-namespace
helm upgrade --install vlcollector vm/victoria-logs-collector --version 0.3.8 \
  -n logging -f vlcollector-values.yaml

# Дашборды
helm upgrade --install grafana grafana-community/grafana --version 14.4.1 \
  -n autoservice -f grafana-values.yaml
