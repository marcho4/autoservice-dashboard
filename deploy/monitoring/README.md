# Мониторинг

Ставится через helm поверх приложения из `deploy/`. Здесь values-файлы релизов и скрипт, который поднимает всё заново.

| Что | Релиз | Чарт | Версия | Namespace | Values |
| --- | --- | --- | --- | --- | --- |
| VictoriaMetrics | `vmsingle` | `vm/victoria-metrics-single` | 0.49.0 | `autoservice` | `vmsingle-values.yaml` |
| vmalert + Alertmanager | `vmalert` | `vm/victoria-metrics-alert` | 0.52.0 | `autoservice` | `vmalert-values.yaml` |
| VictoriaLogs | `vlogs` | `vm/victoria-logs-single` | 0.13.10 | `logging` | по умолчанию |
| Сборщик логов | `vlcollector` | `vm/victoria-logs-collector` | 0.3.8 | `logging` | `vlcollector-values.yaml` |
| Grafana | `grafana` | `grafana-community/grafana` | 14.4.1 | `autoservice` | `grafana-values.yaml` |

## Установка

```sh
./scripts/deploy.sh vX.Y.Z
TELEGRAM_BOT_TOKEN=<токен бота для алертов> ./deploy/monitoring/install.sh
```

Приложение выкатывается отдельно, см. [../README.md](../README.md).
Скрипт можно запускать повторно: `helm upgrade --install` приводит релизы к текущим values.
Токен бота хранится в секрете `alertmanager-telegram`, его нужно передать только при первом запуске или при смене.

Правила алертов лежат в `alerts/`, в кластер они попадают через ConfigMap `deploy/alertConfigMap.yml`.
После правки правил пересоберите ConfigMap и примените его, vmalert подхватит изменения сам:

```sh
kubectl -n autoservice create configmap alerts --from-file=alerts/ --dry-run=client -o yaml > deploy/alertConfigMap.yml
kubectl apply -f deploy/alertConfigMap.yml
```

## Доступ

- VictoriaMetrics: `http://$(minikube ip):30428/vmui` (NodePort `vmui` из `deploy/services.yml`).
- VictoriaLogs: `kubectl -n logging port-forward svc/vlogs-victoria-logs-single-server 9428:9428`, затем http://localhost:9428/select/vmui.
- Grafana: `kubectl -n autoservice port-forward svc/grafana 3001:80`, затем http://localhost:3001, логин `admin`, пароль:
  `kubectl -n autoservice get secret grafana -o jsonpath='{.data.admin-password}' | base64 -d`.
  Источники VictoriaMetrics и VictoriaLogs уже настроены, дашборд «Автосервис: приложение» — в папке «Автосервис».

## Дашборды

JSON дашбордов лежит в `dashboards/`. `install.sh` кладёт их в ConfigMap `grafana-dashboards`, Grafana подхватывает
изменения сама примерно за минуту. Правки в UI Grafana не переживают рестарт пода: меняйте JSON в репозитории
и обновляйте ConfigMap:

```sh
kubectl -n autoservice create configmap grafana-dashboards --from-file=deploy/monitoring/dashboards/ \
  --dry-run=client -o yaml | kubectl apply --server-side --force-conflicts -f -
```

`autoservice.json` устроен сверху вниз: SLO и сводки → RED (Rate, Errors, Duration) по сервисам → USE
(Utilization, Saturation, Errors) по ресурсам → продуктовые показатели. Запросы на MetricsQL, работают только
с VictoriaMetrics.
