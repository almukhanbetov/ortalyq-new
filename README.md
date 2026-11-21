# ORTALYQ-NEW

Учебная платформа для онлайн-занятий:
курс → урок → раздел → упражнение «Тест» → выполнение учеником → домашняя работа → проверка преподавателем → завершение урока с оценкой, плюс чат класса.

Проект независим от старого ORTALYQ: собственный код, отдельная БД, отдельные порты, свой Git-репозиторий без remote.

| Компонент | Технологии | Порт (localhost) |
|---|---|---|
| API | Go 1.26 + Gin, pgx, Goose | 8448 |
| Web | Next.js 15 + TypeScript + Tailwind CSS 3 | 3448 |
| БД | PostgreSQL 16 | 5448 |
| Realtime | WebSocket (`/api/classrooms/:id/ws`) | 8448 |

## Требования

- Docker 24+ с Docker Compose v2 — для запуска одной командой.
- Для разработки без контейнеров: Go ≥ 1.25 (недостающий тулчейн 1.26 Go скачает сам), Node.js ≥ 20, npm.
- Для e2e-проверки интерфейса: установленный Google Chrome или Chromium.

## Быстрый запуск (Docker Compose)

```bash
docker compose up -d --build                 # postgres + api + web; миграции применяются при старте API
docker compose exec api api seed             # демонстрационные данные (только APP_ENV=local)
```

Откройте <http://localhost:3448>.

Список классов: `/cabinet/school/classes` — все; `?type=1` — индивидуальные; `?type=2` — марафоны; `?type=group` — групповые (своё значение: код оригинала неизвестен). Поиск — параметр `?q=`. Старый адрес `/classes` перенаправляется (307) на новый с сохранением параметров. Комната класса осталась по адресу `/classes/:id`.

Остановить: `docker compose down`. Удалить данные: `docker compose down -v`.

## Демонстрационные аккаунты (только локальная среда)

Создаются командой `api seed`. Пароли берутся из переменных `SEED_*_PASSWORD` (см. [backend/.env.example](backend/.env.example)) и в коде не зашиты. Команда `seed` отказывается работать при `APP_ENV=production`.

| Роль | Email | Пароль (локальный по умолчанию) |
|---|---|---|
| Администратор | admin@ortalyq.local | `admin-local-123` |
| Преподаватель | teacher@ortalyq.local | `teacher-local-123` |
| Ученик | student@ortalyq.local | `student-local-123` |
| Ученик 2 | student2@ortalyq.local | `student-local-123` |
| Преподаватель 2 | teacher2@ortalyq.local | `teacher-local-123` |

Seed также создаёт два индивидуальных класса преподавателя, класс второго преподавателя и курсы «English Starter A1» (3 урока с разделами) и «Kazakh for travellers». Упражнения не создаются: это первый шаг проверочного сценария.

## Разработка без контейнеров

```bash
docker compose up -d postgres                       # только БД на 127.0.0.1:5448

cd backend
cp .env.example .env                                # при необходимости поправьте значения
set -a; . ./.env; set +a
go run ./cmd/api migrate up
go run ./cmd/api seed
go run ./cmd/api serve                              # http://localhost:8448

cd ../frontend
cp .env.example .env.local
npm install
npm run dev                                         # http://localhost:3448
```

Сброс локальной демо-БД (удаление схемы → миграции → seed): `./scripts/reset-demo-db.sh`. Скрипт отказывается работать с БД, отличной от локальной на порту 5448.

## Миграции

Миграции Goose лежат в [backend/migrations](backend/migrations) и встроены в бинарник.

```bash
go run ./cmd/api migrate up | down | status | reset
# или CLI goose:
goose -dir backend/migrations postgres "$DATABASE_URL" up
```

При `AUTO_MIGRATE=true` (так в docker-compose) API применяет миграции при старте. На чистой БД проверен цикл up → reset → up.

## Тесты и проверки

```bash
# Backend: unit + интеграционные тесты против реального PostgreSQL (отдельная БД)
docker compose exec postgres psql -U ortalyq_new -d ortalyq_new -c "CREATE DATABASE ortalyq_new_test"   # один раз
cd backend
go vet ./...
TEST_DATABASE_URL='postgres://ortalyq_new:ortalyq_new_local@localhost:5448/ortalyq_new_test?sslmode=disable' go test ./...

# Frontend
cd frontend
npm run typecheck
npm run lint
npm run build

# E2E в браузере: две отдельные сессии, преподаватель и ученик, на свежей демо-БД, при запущенных API и web
./scripts/reset-demo-db.sh
cd frontend && CHROME_PATH=/usr/bin/google-chrome node scripts/e2e.mjs   # скриншоты → docs/screenshots
node scripts/sidebar-check.mjs                                          # левое меню: раскрытие, свёрнутый вид, мобильная панель
```

Без `TEST_DATABASE_URL` интеграционные тесты пропускаются. Они пересоздают схему тестовой БД, поэтому указывайте только отдельную тестовую базу.

Что покрыто, описано в [docs/verification.md](docs/verification.md).

## GIF (GIPHY)

GIF ищутся и проверяются на сервере. Ключ в браузер не попадает.

1. Получите ключ на <https://developers.giphy.com/>.
2. Укажите его в `GIPHY_API_KEY`: в `backend/.env` или в переменной окружения перед `docker compose up` (compose передаёт `GIPHY_API_KEY` в контейнер API).

Без ключа панель «Реакции» показывает понятное сообщение о недоступности, а сервер отвечает `503 giphy_not_configured`. Успешная отправка не имитируется. Путь отправки GIF проверен интеграционным тестом с подменённым сервером GIPHY. С настоящим GIPHY проверка не проводилась: ключа нет.

## Вложения

- Лимит размера: `UPLOAD_MAX_MB` (по умолчанию 10 МБ).
- Тип определяется по содержимому файла: PNG, JPEG, GIF, WebP, PDF, TXT, MP3, MP4, а также DOCX/XLSX/PPTX (ZIP-контейнер с соответствующим расширением). HTML, скрипты и исполняемые файлы отклоняются.
- Имя на диске генерируется случайно, исходное имя хранится только в БД.
- Каталог `UPLOAD_DIR` (в compose — том `uploads` в `/data/uploads`) не раздаётся напрямую. Скачивание идёт только через `GET /api/attachments/:id` с проверкой членства в классе, `Content-Disposition: attachment`, `nosniff` и `CSP: sandbox`.
- При удалении сообщения его вложение удаляется вместе с файлом.

## Конфигурация

Переменные описаны в [backend/.env.example](backend/.env.example) и [frontend/.env.example](frontend/.env.example). Для production:

- `APP_ENV=production`. При этом обязателен `COOKIE_SECURE=true`, иначе API не стартует.
- Свой `DATABASE_URL`, `CORS_ORIGINS` — origin фронтенда.
- `NEXT_PUBLIC_API_URL` передаётся при сборке frontend.
- Переменные `SEED_*` не задавайте.

## Известные ограничения

- Внутренние экраны Edvibe в задании отсутствовали. Интерфейс собран по публичному сайту edvibe.com: шрифт, палитра, превью экрана «Классы». Подробности в [docs/visual-verification.md](docs/visual-verification.md). Соответствие один к одному **не подтверждено**.
- Рабочий тип упражнения только «Тест». Остальные типы в каталоге помечены «Недоступно» и не открываются.
- Начисление баллов работает только по правильным ответам. Шкала ошибок и ручное оценивание показаны как недоступные.
- Классы только индивидуальные (1 преподаватель + 1 ученик). Модель участников (`classroom_members`) готова к групповым.
- Пункты меню «Добро пожаловать», «Ученики», «Расписание», «Сайт и заявки», «Тарифы», «Настройки» ведут на честную страницу «Раздел в разработке». Рабочие пункты — «Классы», «Материалы», «Администрирование» (у администратора).
- Видеосвязь, доска, словарь и таймер класса в панели инструментов показаны недоступными: значок замка и подпись «Недоступно на этом этапе».
- Индикаторы доставки в чате: «Отправляется…» и «Не отправлено». Отметки «прочитано» нет, потому что нет подтверждающих событий.
- При возврате ДЗ на доработку таймер теста не перезапускается: срок снимается.
- WebSocket-хаб работает в одном процессе. Для нескольких экземпляров API понадобится общий брокер (например, PostgreSQL LISTEN/NOTIFY).
- Адаптация для мобильных экранов (390 px) сделана, но с Edvibe не сравнивалась: референсов нет.
- Git-репозиторий создан локально, remote не настроен, коммиты не делались.
