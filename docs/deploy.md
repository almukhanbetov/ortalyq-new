# Production: bilimway.kz

Стек: Caddy (HTTPS) → Next.js (`web`) и Go API (`api`) → PostgreSQL. Всё работает в Docker на одном VPS, каталог `/opt/bilimway`.

Образы собирает GitHub Actions и публикует в GHCR с тегом полного SHA коммита. На сервере ничего не собирается.

```
Интернет ──80/443──► caddy ─┬─ /api/*, /healthz, WebSocket ─► api:8448 ─► postgres:5432
                            └─ всё остальное ───────────────► web:3448
www.bilimway.kz ─► 301 на https://bilimway.kz
```

| Файл в репозитории | На сервере | Назначение |
|---|---|---|
| `deploy/compose.prod.yml` | `/opt/bilimway/compose.yml` | сервисы, тома, сети |
| `deploy/Caddyfile` | `/opt/bilimway/Caddyfile` | HTTPS, www → apex, прокси API/WebSocket |
| `deploy/.env.production.example` | образец для `/opt/bilimway/.env` | настройки без секретов |
| `deploy/scripts/*` | `/opt/bilimway/scripts/` | `deploy.sh`, `healthcheck.sh`, `backup.sh`, `restore.sh`, `rollback.sh`, `dc` |
| `.github/workflows/ci-deploy.yml` | — | проверки → образы → деплой |

Workflow копирует файлы из `deploy/` на сервер при каждом деплое. `.env` на сервере он не трогает: секреты приложения живут только там.

## Безопасность конфигурации

- **БД наружу не опубликована.** У `postgres` нет `ports`, он подключён только к сети `db` с `internal: true`. Эта сеть не имеет выхода наружу, в ней только `migrate` и `api`.
- **API тоже без опубликованного порта.** Снаружи доступны только 80 и 443 у Caddy.
- **Cookie сессии** `__Host-bilimway_session` выставляется с `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/` и без `Domain`. Браузер принимает такую cookie только по HTTPS и только для `bilimway.kz`. При `APP_ENV=production` API не стартует без `COOKIE_SECURE=true`.
- **Один origin.** `https://bilimway.kz` обслуживает и сайт, и API, и WebSocket (`wss://bilimway.kz/api/classrooms/:id/ws`). `CORS_ORIGINS` и `APP_URL` равны `SITE_URL`. POST с чужим `Origin` получает 403, WebSocket принимает только `Origin` этого хоста. `www` перенаправляется на основной домен, поэтому сессия не делится между двумя хостами.
- **IP клиента** для лимитов входа берётся из `X-Forwarded-For` только от доверенных прокси (`TRUSTED_PROXIES`, частные сети Docker). Caddy отбрасывает присланные клиентом `X-Forwarded-*`.
- **Миграции** выполняет отдельный одноразовый сервис `migrate` (`api migrate up`) до старта API. `AUTO_MIGRATE=false`.
- **`api seed` в production отказывается работать.** Первый администратор создаётся командой `api create-admin`, пароль читается из stdin.
- **Почта:** `MAIL_DRIVER=log`. Письма остаются в `email_outbox`, ничего не отправляется. Настоящая почта (П-1…П-5) — отдельный этап.

## GitHub: Secrets и Variables

Settings → Secrets and variables → Actions. Секреты можно завести на уровне репозитория или окружения `production`: job деплоя видит оба уровня.

| Secret | Значение |
|---|---|
| `VPS_HOST` | `83.141.106.42` |
| `VPS_PORT` | `22` |
| `VPS_USER` | `devops` |
| `VPS_SSH_KEY` | приватный ключ деплоя (ed25519, без пароля), целиком с `-----BEGIN…` / `-----END…` |
| `VPS_KNOWN_HOSTS` | строки `ssh-keyscan` сервера, сверенные с отпечатком (шаг 2) |

| Variable (необязательно) | По умолчанию |
|---|---|
| `SITE_URL` | `https://bilimway.kz`. Встраивается в образ web и должен совпадать с `SITE_URL` в `/opt/bilimway/.env`. |

`GITHUB_TOKEN` создаётся автоматически. Им workflow публикует образы, а сервер скачивает их во время деплоя: workflow выполняет `docker login` на VPS токеном job и `docker logout` после. Постоянного токена GHCR на сервере нет.

Пароль БД, ключ GIPHY и прочие настройки приложения в GitHub **не хранятся**. Они есть только в `/opt/bilimway/.env` на сервере.

## Первый запуск

### 1. Ключ деплоя (на своей машине)

```bash
ssh-keygen -t ed25519 -N '' -C 'github-actions-deploy@bilimway' -f ~/.ssh/bilimway_deploy
ssh-copy-id -i ~/.ssh/bilimway_deploy.pub -p 22 devops@83.141.106.42
ssh -i ~/.ssh/bilimway_deploy devops@83.141.106.42 'docker version --format {{.Server.Version}} && docker compose version'
```

Содержимое `~/.ssh/bilimway_deploy` — это secret `VPS_SSH_KEY`.

### 2. Отпечаток хоста → `VPS_KNOWN_HOSTS`

```bash
ssh-keyscan -p 22 -t ed25519,ecdsa,rsa 83.141.106.42 2>/dev/null > bilimway_known_hosts
ssh-keygen -lf bilimway_known_hosts        # отпечатки на вашей стороне
ssh devops@83.141.106.42 'for f in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf "$f"; done'   # и на сервере: должны совпасть
```

Содержимое `bilimway_known_hosts` — secret `VPS_KNOWN_HOSTS`. Workflow подключается с `StrictHostKeyChecking yes`.

### 3. Каталог и `.env` на сервере

`/opt` принадлежит root, поэтому каталог один раз создаёт пользователь с sudo:

```bash
ssh devops@83.141.106.42
sudo install -d -o devops -g devops -m 755 /opt/bilimway
```

Файлы `deploy/` приедут с первым деплоем. `.env` нужно создать заранее, иначе `deploy.sh` остановится. Скопируйте `deploy/.env.production.example` на сервер одним из способов:

```bash
scp deploy/.env.production.example devops@83.141.106.42:/opt/bilimway/   # со своей машины
# или на сервере: curl -fsSL https://raw.githubusercontent.com/almukhanbetov/ortalyq-new/main/deploy/.env.production.example -o /opt/bilimway/.env.production.example
```

Затем на сервере:

```bash
cd /opt/bilimway
cp .env.production.example .env && chmod 600 .env
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 32)/" .env
nano .env        # ACME_EMAIL — ваш адрес для Let's Encrypt; при необходимости GIPHY_API_KEY
```

Пароль БД задаётся при первой инициализации тома `pgdata`. Если позже поменять его в `.env`, PostgreSQL его не подхватит: пароль меняют через `ALTER USER`.

### 4. Порты и DNS

```bash
dig +short bilimway.kz A; dig +short www.bilimway.kz A        # 83.141.106.42
dig +short bilimway.kz AAAA; dig +short www.bilimway.kz AAAA  # пусто или IPv6 этого VPS
sudo ss -ltnp | grep -E ':(80|443)\b'                          # порты свободны
sudo ufw status                                                # если ufw включён: allow 22/tcp, 80/tcp, 443/tcp, 443/udp
```

- Если AAAA указывает на чужой адрес, Let's Encrypt не выдаст сертификат.
- Docker публикует порты в обход ufw. Наружу открыты только 80 и 443 у Caddy, у остальных сервисов портов нет.

### 5. Запуск через GitHub Actions

1. Заведите secrets из таблицы выше.
2. Сделайте push в `main` или запустите workflow вручную (Actions → «CI and deploy» → Run workflow, ветка `main`).
3. Порядок работы: проверки backend и frontend → образы `ghcr.io/almukhanbetov/ortalyq-new/{api,web}:<sha>` → копирование файлов на сервер → `scripts/deploy.sh <sha>` → проверка `https://bilimway.kz/healthz` из интернета.

Что делает `deploy.sh`:

1. `pull` образов.
2. Старт `postgres`.
3. Бэкап БД и загрузок, если это не первый релиз.
4. `migrate`.
5. Старт `api`, `web`, `caddy` с ожиданием healthcheck.
6. `caddy reload`.
7. `healthcheck.sh`: контейнеры, HTTPS, `/healthz`, `/login`, `/api/auth/me` = 401, редиректы `http → https` и `www → apex`.
8. Запись релиза в `current_tag` и `releases.log`.

Если контейнеры не стали здоровыми или проверка не прошла, API и web возвращаются на предыдущий релиз автоматически.

При первом запуске Caddy получает сертификаты Let's Encrypt. Проверка ждёт до 3 минут.

### 6. Первый администратор

```bash
cd /opt/bilimway
read -rsp 'Пароль администратора: ' PW; echo
printf '%s\n' "$PW" | ./scripts/dc run --rm --no-deps -T api create-admin admin@bilimway.kz "Администратор"; unset PW
```

Пароль не попадает ни в историю shell, ни в аргументы процесса. Существующий email команда не перезаписывает. Дальше преподаватели и ученики создаются в интерфейсе: «Администрирование».

### 7. Проверка после запуска

```bash
cd /opt/bilimway
./scripts/dc ps                       # postgres, api, web — healthy; caddy — running; migrate — exited (0)
./scripts/healthcheck.sh
./scripts/dc logs --tail=50 api caddy
curl -sI https://www.bilimway.kz | grep -i location      # https://bilimway.kz/
```

В браузере:

1. Войдите на `https://bilimway.kz/login`.
2. В DevTools → Application → Cookies должна быть `__Host-bilimway_session` с флагами Secure и HttpOnly.
3. Откройте класс: в Network → WS — соединение `wss://bilimway.kz/api/classrooms/…/ws` со статусом 101.

**IP клиентов.** После входа посмотрите, какой IP записан:

```bash
./scripts/dc exec -T postgres psql -U bilimway -d bilimway -c "select ip, attempted_at from login_attempts order by id desc limit 5"
```

Если вместо внешнего адреса видно `172.x.x.x` (шлюз Docker), значит соединения проходят через userland-proxy Docker. Так бывает с IPv6-клиентами. Тогда лимит входа по IP становится общим для всех таких клиентов. Варианты исправления:

- `"userland-proxy": false` в `/etc/docker/daemon.json` и перезапуск Docker;
- либо `network_mode: host` для Caddy.

Это отдельное изменение на сервере.

## Ежедневная работа

```bash
cd /opt/bilimway
./scripts/dc ps
./scripts/dc logs -f --tail=100 api
./scripts/dc exec postgres psql -U bilimway -d bilimway
cat current_tag; tail releases.log
```

`scripts/dc` — это `docker compose` с тегом текущего релиза. Вызывать `docker compose` напрямую не нужно: без `IMAGE_TAG` он откажется работать.

## Резервное копирование

```bash
./scripts/backup.sh daily          # backups/db-<UTC>-daily.dump + backups/uploads-<UTC>-daily.tar.gz
```

- **БД** — `pg_dump -Fc`, проверяется `pg_restore --list`. **Загрузки** — tar.gz тома `uploads`.
- Хранится по 14 последних копий каждого вида (`BACKUP_KEEP`). Каталог `backups/` закрыт правами 700, файлы — 600.
- `deploy.sh` делает копию `pre-<sha>` перед миграциями каждого релиза.
- Ежедневно по cron (`crontab -e` у devops):

  ```
  30 2 * * * /opt/bilimway/scripts/backup.sh daily >> /opt/bilimway/backups/backup.log 2>&1
  ```
- **Копии на том же VPS не защищают от потери сервера.** Регулярно забирайте их наружу, например со своей машины: `rsync -a devops@83.141.106.42:/opt/bilimway/backups/ ./bilimway-backups/`. Или настройте выгрузку в объектное хранилище (отдельная задача).
- Тома `caddy_data` (сертификаты) и `pgdata` нельзя удалять: `docker compose down -v` уничтожит БД.

## Откат

```bash
./scripts/rollback.sh                # на предыдущий релиз из releases.log
./scripts/rollback.sh <commit-sha>   # на конкретный
```

- Откатываются только образы `api` и `web`. Сервис `migrate` при откате не запускается.
- **Миграции не откатываются.** Если отменяемый релиз добавил миграции, с которыми старый код не работает, восстановите БД из копии `pre-<sha>`, снятой перед этим релизом:

  ```bash
  ls backups/*pre-<первые 12 символов sha>*
  ./scripts/rollback.sh <старый sha>
  ./scripts/restore.sh backups/db-…-pre-<sha12>.dump backups/uploads-…-pre-<sha12>.tar.gz
  ```

  Данные, внесённые после этой копии, будут потеряны. Перед заменой `restore.sh` сам снимает копию `pre-restore`, запрашивает подтверждение словом `RESTORE` и останавливает api и web на время восстановления.
- Образы прошлых релизов остаются на сервере: `deploy.sh` удаляет только неиспользуемые слои. Если образа нет, `rollback.sh` скачает его. Для этого нужен `docker login ghcr.io` с токеном `read:packages` или публичные пакеты.
- Повторный деплой нужного коммита: Actions → «CI and deploy» → Re-run для этого коммита.

## Что проверено локально

Стек из этих файлов поднимался локально в изолированном compose-проекте:

- образы взяты из временного registry;
- Caddy работал с внутренним CA для `localhost:8471`;
- порты были привязаны к 127.0.0.1;
- dev-БД и демо-стенд не затрагивались.

| Проверка | Результат |
|---|---|
| Первый деплой `deploy.sh`: миграции до 17, healthcheck | ok |
| `create-admin`: создание, отказ для существующего email и слабого пароля; `seed` в production отказывает | ok |
| У `postgres` и `api` нет опубликованных портов, сеть `db` internal | ok |
| Вход через UI, cookie `__Host-bilimway_session` (Secure, HttpOnly, Lax, host-only), `/me`, обновление страницы, выход | ok |
| WebSocket класса через Caddy (`wss://…/ws`), событие чата доходит до ученика | ok |
| Загрузка и скачивание файла; 11 МБ — 413 от API; 20 МБ — 413 от Caddy; аноним — 401 | ok |
| `http → https` (308), `www → apex` (301), HSTS; POST с чужим Origin — 403; подставной `X-Forwarded-For` игнорируется | ok |
| Повторный деплой с бэкапом `pre-<sha>`, `rollback.sh`, `restore.sh` (данные после копии исчезли, файлы восстановлены) | ok |
| Релиз с нездоровым web → автоматический возврат на предыдущий; отсутствующий образ → остановка до изменений | ok |
| `shellcheck` скриптов, `actionlint` workflow | ok |

На настоящем VPS не проверялись: выпуск сертификатов Let's Encrypt, публикация в GHCR и деплой из GitHub Actions.
