# Выкатка

Версия — это git-тег `vX.Y.Z`. По тегу GitHub Actions собирает два образа в GHCR, кластер тянет их сам:
`ghcr.io/marcho4/autoservice-backend:vX.Y.Z` и `ghcr.io/marcho4/autoservice-frontend:vX.Y.Z`.

```sh
./scripts/release.sh v1.2.3   # тег на текущий коммит main; CI гоняет тесты и собирает образы
./scripts/deploy.sh v1.2.3    # когда сборка позеленела: манифесты, миграции, ожидание подов
./scripts/deploy.sh v1.2.2    # откат — выкатка предыдущей версии
```

`deploy.sh` работает с тем кластером, на который смотрит текущий контекст `kubectl`, и перед выкаткой
показывает его и переход версий. Запускать можно с ноутбука (с kubeconfig виртуалки) или на самой виртуалке
из клона репозитория.

Что происходит при выкатке:

- применяются `config-map.yml`, `alertConfigMap.yml`, `services.yml` и `deployments.yml`, где `VERSION` заменён на тег;
- у каждого нового пода бэкенда init-контейнер выполняет `app migrate` из того же образа;
- поды меняются по одному и получают трафик только после ответа `/readyz` (бэкенд) и `/healthz` (фронт);
- если новая версия не поднялась за 5 минут, скрипт завершается с ошибкой и печатает команду отката;
  прежние поды всё это время продолжают работать.

Миграции назад не откатываются, поэтому предыдущая версия кода должна работать с новой схемой:
колонку или таблицу удаляйте версией позже, чем код перестал ими пользоваться.

Правка `config-map.yml` сама поды не перезапускает. Если версия та же, после `deploy.sh` выполните
`kubectl -n autoservice rollout restart deployment/backend deployment/frontend`.

## Первый запуск на новой виртуалке

1. Кластер. Для одной виртуалки подойдёт k3s; вторая нода потом добавляется одной командой.

   ```sh
   curl -sfL https://get.k3s.io | sh -                  # на виртуалке
   sudo cat /etc/rancher/k3s/k3s.yaml                   # kubeconfig; для ноутбука замените 127.0.0.1 на адрес виртуалки
   ```

2. Доступ к образам. После первой сборки пакеты `autoservice-backend` и `autoservice-frontend` появятся в GHCR
   приватными. Проще всего сделать их публичными: GitHub → профиль → Packages → пакет → Package settings →
   Change visibility. Если они должны остаться приватными, дайте кластеру токен с правом `read:packages`:

   ```sh
   kubectl create namespace autoservice
   kubectl -n autoservice create secret docker-registry ghcr \
     --docker-server=ghcr.io --docker-username=<логин GitHub> --docker-password=<токен>
   kubectl -n autoservice patch serviceaccount default -p '{"imagePullSecrets":[{"name":"ghcr"}]}'
   ```

3. Секреты: `LLM_API_KEY=<ключ> ./scripts/init-secrets.sh`. Пароль БД и ключ сессий скрипт генерирует сам.
4. Настройки: в `config-map.yml` впишите `APP_URL`, для https поставьте `COOKIE_SECURE: "true"`.
5. `./scripts/deploy.sh vX.Y.Z`. Кабинет откроется на `http://<адрес виртуалки>:30080`.
6. Мониторинг: [monitoring/README.md](monitoring/README.md).
