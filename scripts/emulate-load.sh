#!/usr/bin/env bash
# Эмуляция работы платформы в minikube, чтобы на графиках VictoriaMetrics было что смотреть:
# Telegram-боты создают заявки, клиенты их правят и отменяют, сотрудники разбирают их на дашборде.
# Плюс немного типичных ошибок клиентов (400/401/404/409/422).
#
#   ./scripts/emulate-load.sh             # 5 минут нагрузки
#   DURATION=0 ./scripts/emulate-load.sh  # до Ctrl+C
#   CHAOS=1 ./scripts/emulate-load.sh     # на трети прогона гасит БД на CHAOS_DOWN секунд: 5xx и падение пробера
#   ./scripts/emulate-load.sh cleanup     # удалить всё, что насоздавал скрипт
#
# Настройки через env:
#   NS=autoservice   SERVICES=3   BOT_WORKERS=4   EMP_WORKERS=2
#   DURATION=300     PAUSE=0.2    LOCAL_PORT=18080
#   CHAOS=0          CHAOS_DOWN=30

set -uo pipefail

NS=${NS:-autoservice}
SERVICES=${SERVICES:-3}
BOT_WORKERS=${BOT_WORKERS:-4}
EMP_WORKERS=${EMP_WORKERS:-2}
DURATION=${DURATION:-300}
PAUSE=${PAUSE:-0.2}
LOCAL_PORT=${LOCAL_PORT:-18080}
CHAOS=${CHAOS:-0}
CHAOS_DOWN=${CHAOS_DOWN:-30}

BASE=http://127.0.0.1:$LOCAL_PORT
EMAIL_PREFIX=loadtest
PASSWORD=loadtest-password
# Подходит под формат токена бота, но Telegram его отвергнет.
FAKE_BOT_SECRET=AAHloadtestloadtestloadtestloadtest

CARS="Toyota:Camry Toyota:RAV4 Lada:Vesta Lada:Granta Kia:Rio Hyundai:Solaris BMW:X5 Skoda:Octavia Volkswagen:Polo Mercedes-Benz:E-Class"
NAMES="Иван Пётр Анна Мария Сергей Ольга Дмитрий Елена"
PROBLEMS=("Замена масла и фильтров" "Стук в передней подвеске" "Не заводится в мороз" "Скрипят тормоза"
  "Плановое ТО" "Горит check engine" "Сезонный шиномонтаж" "Не работает кондиционер")

TOKEN='' CODE='' BODY='' PF_PID='' CHAOS_PID='' WORKER_PIDS='' PREV_TOTAL=0

log() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
die() {
  log "ошибка: $*"
  exit 1
}

kc() { kubectl -n "$NS" "$@"; }
secret() { kc get secret app-secret -o "jsonpath={.data.$1}" | base64 -d; }
psql_db() { kc exec db-0 -c postgres -- psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -qtAc "$1"; }

pick() {
  shift $((RANDOM % $#))
  printf '%s' "$1"
}
chance() { [ $((RANDOM % 100)) -lt "$1" ]; }

# call METHOD PATH [BODY [curl-args...]]: тело ответа в $BODY, HTTP-код в $CODE (000 — бэкенд не ответил).
call() {
  local method=$1 path=$2 data=${3:-} out
  shift $(($# < 3 ? $# : 3))
  out=$(curl -s -m 15 -X "$method" "$BASE$path" -H 'Content-Type: application/json' \
    ${data:+--data-binary "$data"} "$@" -w '\n%{http_code}') || true
  CODE=${out##*$'\n'}
  BODY=${out%$'\n'*}
  echo "$CODE" >>"$STATS"
}
emp() { call "$1" "/api/v1$2" "${3:-}" -H "Authorization: Bearer $TOKEN"; }
runner() { call "$1" "/internal/v1$2" "${3:-}" -H "X-API-Key: $BOT_API_KEY"; }

# Случайный id из ответа вида {"items": [...]}, аргумент — jq-фильтр элементов.
rand_id() {
  jq -r --argjson n "$RANDOM" "[.items[]? | ${1:-.}] | if length > 0 then .[\$n % length].id else empty end" \
    <<<"$BODY" 2>/dev/null
}

session() {
  TOKEN=$(jq -r '.access_token // empty' <<<"$BODY" 2>/dev/null)
  [ -n "$TOKEN" ]
}
login() {
  call POST /api/v1/auth/login "{\"email\":\"$1\",\"password\":\"$PASSWORD\"}"
  session
}
register() {
  call POST /api/v1/auth/register "{\"email\":\"$1\",\"password\":\"$PASSWORD\",\"full_name\":\"$2\"}"
  session
}

phone() { printf '+7 9%02d %03d-%02d-%02d' $((RANDOM % 100)) $((RANDOM % 1000)) $((RANDOM % 100)) $((RANDOM % 100)); }

# Поля заявки без фигурных скобок: при создании их склеивают с "client".
request_fields() {
  local car
  car=$(pick $CARS)
  printf '"car_brand":"%s","car_model":"%s","description":"%s","phone":"%s"' \
    "${car%%:*}" "${car#*:}" "$(pick "${PROBLEMS[@]}")" "${1:-$(phone)}"
}

bot_worker() {
  local bot=$1 tg client id r
  while :; do
    tg=$((700000000 + RANDOM % 300)) # у каждого бота ~300 постоянных клиентов
    client="\"client\":{\"telegram_id\":$tg,\"telegram_username\":\"user$tg\",\"name\":\"$(pick $NAMES)\"}"
    r=$((RANDOM % 100))
    if [ "$r" -lt 55 ]; then
      if chance 3; then
        runner POST "/bots/$bot/requests" "{$client,$(request_fields 'не скажу')}" # 400: кривой телефон
      else
        runner POST "/bots/$bot/requests" "{$client,$(request_fields)}"
      fi
    elif [ "$r" -lt 85 ]; then
      runner GET "/bots/$bot/clients/$tg/requests"
      id=$(rand_id 'select(.status == "new")')
      if [ -n "$id" ]; then
        runner GET "/bots/$bot/clients/$tg/requests/$id"
        if chance 40; then
          runner PATCH "/bots/$bot/clients/$tg/requests/$id" "{$(request_fields)}"
        elif chance 30; then
          runner POST "/bots/$bot/clients/$tg/requests/$id/cancel"
        fi
      fi
    elif [ "$r" -lt 97 ]; then
      runner GET /bots # раннер перечитывает список ботов
    elif [ "$r" -lt 99 ]; then
      runner GET "/bots/1/clients/$tg/requests" # 404: такого бота нет
    else
      call GET /internal/v1/bots "" -H 'X-API-Key: wrong-key' # 401
    fi
    sleep "$PAUSE"
  done
}

emp_worker() {
  local email=$1 id r
  login "$email"
  while :; do
    emp GET "/requests?status=new&limit=20" # дашборд опрашивает новые заявки
    if [ "$CODE" = 401 ]; then
      login "$email"
      sleep "$PAUSE"
      continue
    fi
    id=$(rand_id)
    if [ -n "$id" ]; then
      emp GET "/requests/$id"
      r=$((RANDOM % 100))
      if [ "$r" -lt 35 ]; then
        emp POST "/requests/$id/take"
      elif [ "$r" -lt 65 ]; then
        emp PATCH "/requests/$id/status" '{"status":"in_progress"}'
      elif [ "$r" -lt 85 ]; then
        emp POST "/requests/$id/reject"
      elif [ "$r" -lt 90 ]; then
        emp POST "/requests/$id/close" # 409: из new сразу закрыть нельзя
      fi
    fi
    if chance 50; then
      emp GET "/requests?status=in_progress&limit=20"
      id=$(rand_id)
      [ -n "$id" ] && emp POST "/requests/$id/close"
    fi

    r=$((RANDOM % 100))
    if [ "$r" -lt 15 ]; then
      emp GET /clients
      id=$(rand_id)
      [ -n "$id" ] && emp GET "/clients/$id"
    elif [ "$r" -lt 25 ]; then
      emp GET /me
      emp GET /autoservice
      emp GET /autoservice/bot
    elif [ "$r" -lt 28 ]; then
      emp GET "/requests?sort=asc&limit=50&offset=$((RANDOM % 100))"
    elif [ "$r" -lt 33 ]; then
      login "$email"
    elif [ "$r" -lt 36 ]; then
      call POST /api/v1/auth/login "{\"email\":\"$email\",\"password\":\"wrong-password\"}" # 401
    elif [ "$r" -lt 38 ]; then
      call POST /api/v1/auth/register "{\"email\":\"$email\",\"password\":\"$PASSWORD\",\"full_name\":\"Дубль\"}" # 409
    elif [ "$r" -lt 39 ]; then
      call POST /api/v1/auth/register \
        "{\"email\":\"$EMAIL_PREFIX-$(date +%s)-$RANDOM@example.com\",\"password\":\"$PASSWORD\",\"full_name\":\"Новый сотрудник\"}"
    elif [ "$r" -lt 41 ]; then
      emp PUT /autoservice/bot "{\"token\":\"$((RANDOM + 1000)):$FAKE_BOT_SECRET\"}" # 422 от Telegram, растут tg_* метрики
    fi
    sleep "$PAUSE"
  done
}

setup_services() {
  local run i email as_id bot_id
  run=$(date +%s)
  : >"$WORK_DIR/services"
  for ((i = 1; i <= SERVICES; i++)); do
    email="$EMAIL_PREFIX-$run-$i@example.com"
    register "$email" "Нагрузочный сотрудник $i" || die "регистрация $email: $CODE $BODY"
    emp POST /autoservice "{\"name\":\"Автосервис №$i\",\"address\":\"ул. Нагрузочная, $i\",\"phone\":\"$(phone)\",\"description\":\"создан emulate-load.sh\"}"
    [ "$CODE" = 201 ] || die "создание автосервиса: $CODE $BODY"
    as_id=$(jq -r .id <<<"$BODY")
    # Через API бота не подключить: бэкенд проверяет токен в api.telegram.org. Поэтому кладём его прямо в БД.
    bot_id=$((run * 100 + i))
    psql_db "INSERT INTO bots (autoservice_id, telegram_bot_id, username, token)
             VALUES ('$as_id', $bot_id, 'loadtest_${bot_id}_bot', '$bot_id:$FAKE_BOT_SECRET')" >/dev/null ||
      die "не удалось завести бота в БД"
    echo "$email $bot_id" >>"$WORK_DIR/services"
    log "автосервис $i: сотрудник $email, бот $bot_id"
  done
}

# Сервис ClusterIP, поэтому ходим через port-forward. Он держит соединение с одним подом backend,
# так что метрики будут расти только на нём — для графиков с sum() это не важно.
start_port_forward() {
  local i
  kc port-forward svc/backend "$LOCAL_PORT:8080" >>"$WORK_DIR/port-forward.log" 2>&1 &
  PF_PID=$!
  for ((i = 0; i < 50; i++)); do
    curl -sf -m 1 "$BASE/healthz" >/dev/null && return
    sleep 0.2
  done
  die "port-forward к backend не поднялся: $(tail -n 3 "$WORK_DIR/port-forward.log")"
}

chaos() {
  sleep "$1"
  log "CHAOS: гашу БД на ${CHAOS_DOWN}с"
  kc scale statefulset db --replicas=0 >/dev/null
  sleep "$CHAOS_DOWN"
  log "CHAOS: поднимаю БД"
  kc scale statefulset db --replicas=1 >/dev/null
}

report() {
  local total ok c4 c5 down
  read -r total ok c4 c5 down < <(awk '{n++} /^2/{a++} /^4/{b++} /^5/{c++} /^000/{d++}
    END {printf "%d %d %d %d %d\n", n, a, b, c, d}' "$STATS")
  log "${1:-~$(((total - PREV_TOTAL) / 10)) rps} | запросов $total | 2xx $ok | 4xx $c4 | 5xx $c5 | без ответа $down"
  PREV_TOTAL=$total
}

on_exit() {
  trap - EXIT
  [ -n "$WORKER_PIDS" ] && kill $WORKER_PIDS 2>/dev/null
  [ -n "$CHAOS_PID" ] && kill "$CHAOS_PID" 2>/dev/null && kc scale statefulset db --replicas=1 >/dev/null
  [ -n "$PF_PID" ] && kill "$PF_PID" 2>/dev/null
  wait 2>/dev/null
  report итого
  rm -rf "$WORK_DIR"
  log "графики: http://$(minikube ip 2>/dev/null || echo '<minikube ip>'):30428/vmui/"
  log "удалить тестовых сотрудников и их данные: $0 cleanup"
}

command -v kubectl >/dev/null && command -v curl >/dev/null && command -v jq >/dev/null ||
  die "нужны kubectl, curl и jq"
PG_USER=$(secret POSTGRES_USER) && PG_DB=$(kc get configmap app-config -o 'jsonpath={.data.POSTGRES_DB}') &&
  [ -n "$PG_USER" ] || die "не удалось прочитать app-secret/app-config из неймспейса $NS — кластер запущен?"

if [ "${1:-}" = cleanup ]; then
  n=$(psql_db "WITH d AS (DELETE FROM employees WHERE email LIKE '$EMAIL_PREFIX-%@example.com' RETURNING 1)
               SELECT count(*) FROM d") || die "cleanup не удался"
  log "удалено сотрудников: $n (их автосервисы, боты, клиенты и заявки удалились каскадом)"
  exit 0
fi

BOT_API_KEY=$(secret BOT_API_KEY)
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/emulate-load.XXXXXX")
STATS=$WORK_DIR/stats
: >"$STATS"
trap on_exit EXIT
trap 'exit 130' INT TERM

start_port_forward
setup_services

service() { awk -v n=$((($1 - 1) % SERVICES + 1)) -v col="$2" 'NR == n {print $col}' "$WORK_DIR/services"; }
for ((k = 1; k <= BOT_WORKERS; k++)); do
  bot_worker "$(service "$k" 2)" &
  WORKER_PIDS="$WORKER_PIDS $!"
done
for ((k = 1; k <= EMP_WORKERS; k++)); do
  emp_worker "$(service "$k" 1)" &
  WORKER_PIDS="$WORKER_PIDS $!"
done

if [ "$CHAOS" = 1 ]; then
  delay=60
  [ "$DURATION" -gt 0 ] && delay=$((DURATION / 3))
  chaos "$delay" &
  CHAOS_PID=$!
fi

log "нагрузка пошла: ботов $BOT_WORKERS, сотрудников $EMP_WORKERS, длительность $([ "$DURATION" -gt 0 ] && echo "${DURATION}с" || echo 'до Ctrl+C')"
deadline=$(($(date +%s) + DURATION))
while [ "$DURATION" -eq 0 ] || [ "$(date +%s)" -lt "$deadline" ]; do
  sleep 10
  kill -0 "$PF_PID" 2>/dev/null || {
    log "port-forward упал, перезапускаю"
    start_port_forward
  }
  report
done
