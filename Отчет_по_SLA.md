# SLA/SLI/SLO

## Выбор метрик

Суть приложения в том, чтобы корректно обрабатывать заявки и отображать их на дашборде.
На основании этого core функционала я беру следующие метрики для наблюдения:

Доступность ручки создания заявок - без нее теряем клиентов. На нее пятисотки и время запроса.
Доля 5xx на логине и регистрации в бэкенде + длительность запроса не больше 500 ms.
Доступность ручек для работы с заявками. Доля 5xx не более процента от всех запросов + длительность запроса не больше 500 ms.

Доступность компонент в целом буду проверять пробером:
- Фронтенд (ручка /healthz)
- Бэкенд (ручка /readyz)

Для пробирования написал `prober.go` и добавил его запуск в кластере через деплоймент.

### Используемые метрики

http_requests_total - кол-во запросов. Есть фильтр по ручке, методу и статусу.
http_request_duration_milliseconds - гистограмма длительности запросов. Есть фильтр по ручке и методу
prober_check - показывает работает ли сервис

Для HTTP сценария считаем метрики по RBSLO. 
Для пробера - TBSLO, так как даунтайм сервисов измеряется в минутах.

## SLI

Для запросов смотрим долю запросов со статусом != 5xx, поделенный на кол-во всех запросов в окне. Смотрим по метрике http_requests_total.
Для запросов смотрим долю запросов, исполненных меньше чем за 500мс, деленные на кол-во всех запросов в окне.
Для сервисов смотрим кол-во минут, которые они находились в поднятом состоянии / всё кол-во минут в окне.

## SLO

Берем окно 30 дней.
Для 99.9% за 30 дней бюджет ≈ 43 минуты, для 99.5% ≈ 3.6 часа.

| Сценарий | Ручки | Доступность (не 5xx) | Latency |
|---|---|---|---|
| Создание заявки ботом | /internal/v1/bots/{botID}/requests | ≥ 99.9% | 99% < 500 мс |
| Работа с заявками | /api/v1/requests.* | ≥ 99.5% | 99% < 500 мс |
| Логин и регистрация | /api/v1/auth/login, /api/v1/auth/register | ≥ 99.5% | 99% < 500 мс |
| Работоспособность сервисов | backend, frontend | ≥ 99.99% | - |

Формула остатка Error budget = 1 - (1 - SLI_WINDOW) / (1 - SLO)

## SLA

Делаю немного мягче SLO
Берем окно 30 дней. Для демонстрации возьму меньше, чтобы были видны движения при коротких сбоях.
Для 99.9% за 30 дней бюджет ≈ 43 минуты, для 99.5% ≈ 3.6 часа, 95% ≈ 36 часов.

| Сценарий | Ручки | Доступность (не 5xx) | Latency |
|---|---|---|---|
| Создание заявки ботом | /internal/v1/bots/{botID}/requests | ≥ 99.5% | 95% < 500 мс |
| Работа с заявками | /api/v1/requests.* | ≥ 99% | 95% < 500 мс |
| Логин и регистрация | /api/v1/auth/login, /api/v1/auth/register | ≥ 99% | 95% < 500 мс |
| Работоспособность сервисов | backend, frontend | ≥ 99.9% | - |


Формула остатка Error budget = 1 - (1 - SLI_WINDOW) / (1 - SLA)

## Формулы для дашборда

```promql
# 1. Пробер: доступность сервисов (SLI), цель — 1
sum(rate(prober_check{result="ok"}[5m])) by (service)
  / sum(rate(prober_check[5m])) by (service)

# 2. Пробер жив (иначе запрос 1 пустой, а не 0)
up{instance=~".*:8081"}

# Сценарий «Создание заявки ботом» (SLO: доступность ≥ 99.9%, 99% < 500 мс)

# 3. Доступность (доля не-5xx)
(sum(rate(http_requests_total{route="/internal/v1/bots/{botID}/requests",status!="5xx"}[5m]))
  or sum(rate(http_requests_total{route="/internal/v1/bots/{botID}/requests"}[5m])) * 0)
/ sum(rate(http_requests_total{route="/internal/v1/bots/{botID}/requests"}[5m]))

# 4. Latency SLI: доля запросов быстрее 500 мс (цель ≥ 0.99)
histogram_share(500, sum(rate(http_request_duration_milliseconds_bucket{route="/internal/v1/bots/{botID}/requests"}[5m])) by (vmrange))

# 5. p99 латентности, мс — для наглядности рядом с порогом 500
histogram_quantile(0.99, sum(rate(http_request_duration_milliseconds_bucket{route="/internal/v1/bots/{botID}/requests"}[5m])) by (vmrange))

# Сценарий «Работа с заявками» (SLO: доступность ≥ 99.5%, 99% < 500 мс)

# 6. Доступность (доля не-5xx)
(sum(rate(http_requests_total{route=~"/api/v1/requests.*",status!="5xx"}[5m]))
  or sum(rate(http_requests_total{route=~"/api/v1/requests.*"}[5m])) * 0)
/ sum(rate(http_requests_total{route=~"/api/v1/requests.*"}[5m]))

# 7. Latency SLI: доля запросов быстрее 500 мс (цель ≥ 0.99)
histogram_share(500, sum(rate(http_request_duration_milliseconds_bucket{route=~"/api/v1/requests.*"}[5m])) by (vmrange))

# 8. p99 латентности, мс
histogram_quantile(0.99, sum(rate(http_request_duration_milliseconds_bucket{route=~"/api/v1/requests.*"}[5m])) by (vmrange))

# Сценарий «Логин и регистрация» (SLO: доступность ≥ 99.5%, 99% < 500 мс)

# 9. Доступность (доля не-5xx)
(sum(rate(http_requests_total{route=~"/api/v1/auth/(login|register)",status!="5xx"}[5m]))
  or sum(rate(http_requests_total{route=~"/api/v1/auth/(login|register)"}[5m])) * 0)
/ sum(rate(http_requests_total{route=~"/api/v1/auth/(login|register)"}[5m]))

# 10. Latency SLI: доля запросов быстрее 500 мс (цель ≥ 0.99)
histogram_share(500, sum(rate(http_request_duration_milliseconds_bucket{route=~"/api/v1/auth/(login|register)"}[5m])) by (vmrange))

# 11. p99 латентности, мс
histogram_quantile(0.99, sum(rate(http_request_duration_milliseconds_bucket{route=~"/api/v1/auth/(login|register)"}[5m])) by (vmrange))

```

## Поведение при сбоях

1. Запустил эмулятор нагрузки `emulate_load.sh`
2. Уронил frontend `kubectl -n autoservice scale deploy frontend --replicas=0`
3. Скрин 1 (поломка_1.png) - доступность фронтенда пошла вниз.
4. DURATION=0 ./scripts/emulate-load.sh - запустил нагрузку и сдул бд до нуля реплик `kubectl -n autoservice scale statefulset db --replicas=0`
5. Смотрим на SLI графики в папке `sla_images`. Все графики пошли вниз, так как приложение перестало работать.
6. После восстановления SLI график вернулся в норму `recovery.png`