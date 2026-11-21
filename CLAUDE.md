# CLAUDE.md — ORTALYQ-NEW

Модульный монолит: Go API + Next.js фронтенд + PostgreSQL. Независим от старого ORTALYQ: его код, БД, конфиги и remote не использовать.

## Архитектура

- `backend/` — Go 1.26, Gin, pgx/v5 (pgxpool; для Goose — pgx/stdlib), Goose, coder/websocket.
- `frontend/` — Next.js 15 App Router, TypeScript strict, Tailwind 3, TanStack Query v5.
- Realtime: WebSocket-хаб в процессе (`internal/realtime`). События — только уведомления. Источник правды — PostgreSQL. После (пере)подключения клиент инвалидирует запросы класса и перечитывает их через REST.
- Шаблоны материалов (`courses/lessons/sections/exercises/*_versions`) отделены от прохождения в классе (`classroom_lessons`, `section_completions`, `attempts`, `homeworks`). Один урок в двух классах не смешивает результаты.
- Упражнение — постоянный идентификатор плюс неизменяемые версии (`exercise_versions → exercise_questions → exercise_options`). Каждое сохранение создаёт новую версию, а попытка ссылается на `version_id`.
- Баллы не копятся, а пересчитываются из ответов (`attempts.Grade`), поэтому повторный запрос или переподключение не начисляет их дважды.

## Структура backend-модуля (`internal/modules/<name>/`)

| Файл | Содержимое |
|---|---|
| `model.go` | доменные типы и константы |
| `dto.go` | запросы и ответы API |
| `repository.go` | только SQL (параметризованный), без бизнес-логики |
| `service.go` | бизнес-логика и проверки прав; **не зависит от Gin**, получает `auth.Principal` |
| `handler.go` | разбор HTTP → вызов сервиса → ответ; без SQL |
| `routes.go` | регистрация маршрутов |

Зависимости между модулями задаются интерфейсами в пакете-потребителе и передаются через конструкторы в `internal/router/router.go`. Пример: `attempts.HomeworkState` реализован в `homework.Repository`.

Модули: `auth` (сессии, вход, роли), `users` (админ, назначения), `classrooms` (классы, члены, текущий урок + общие проверки доступа: `MemberRole`, `RequireLesson`, `ResolveStudent`), `materials`, `exercises` (каталог, версии, валидация теста через `KindHandler`), `attempts` (старт, ответ, проверка, завершение, сброс, баллы, снимки ДЗ), `homework`, `grading` (разделы, урок, оценка, сводка), `chat` (+ GIPHY-клиент), `uploads`.

Ошибки возвращаются как `*apperr.Error` и отдаются в едином формате `{"error":{"code","message","fields"}}` (`httpx.Fail`).

## Frontend

- Маршруты списка классов: `src/lib/routes.ts` (`CLASSES_ROUTE = /cabinet/school/classes`, коды `type`). Не писать путь строкой; фильтр и поиск страницы хранятся в адресе. Редирект `/classes` → новый маршрут задан в `next.config.mjs`.
- `src/app` — только маршруты и страницы. Группы: `(app)` — личный кабинет, `(room)` — полноэкранный класс со своей оболочкой.
- Оболочка кабинета одна: `components/layout/cabinet/` (`CabinetLayout` + `CabinetHeader` + `CabinetSidebar`, пункты меню — `navigation.tsx`). Она подключается один раз в `app/(app)/layout.tsx`. Страницы кабинета не рендерят шапку или меню сами. Поведение меню — `hooks/useSidebar.ts`; проверка — `frontend/scripts/sidebar-check.mjs`.
- `src/features/<domain>` — предметные компоненты.
- `src/components/ui` — общий UI-кит.
- `src/lib/api` — единый клиент (`client.ts`) и типизированные эндпоинты (`endpoints.ts`).
- `src/types/api.ts` — DTO.
- `src/lib/queryKeys.ts` — ключи React Query.
- Серверное состояние живёт только в React Query, временное состояние интерфейса — в `useState`.
- Строки интерфейса — только в `src/locales/{ru,kk,en}.json` (ru основной). Набор ключей во всех трёх файлах должен совпадать.
- Дизайн-токены — `src/styles/tokens.css` + `tailwind.config.ts`. Не вводить произвольные цвета в обход токенов.

## Команды

```bash
docker compose up -d --build && docker compose exec api api seed   # весь стек
./scripts/reset-demo-db.sh                                         # чистая демо-БД (локально)
cd backend && go vet ./... && TEST_DATABASE_URL=...ortalyq_new_test go test ./...
cd frontend && npm run typecheck && npm run lint && npm run build
cd frontend && node scripts/e2e.mjs                                # браузерный сценарий + скриншоты
```

## Правила разработки

- Права проверяются на сервере в сервисе. Скрытие кнопок на фронтенде — только UX.
- Ученик не получает `is_correct`, пока это не разрешено (`Version.Sanitized()`; раскрытие после завершения или для мгновенно проверенного вопроса).
- Время попыток считается по `now()` БД. Клиент лишь отображает отсчёт с поправкой на `server_now`.
- Важные переходы идемпотентны: старт попытки, завершение, отправка ДЗ, смена статуса, сообщения по `client_id`.
- Связанные изменения выполняются в транзакции (`database.WithTx`), строки блокируются через `FOR UPDATE`.
- Новая миграция — новый файл `NNNNN_name.sql` с секциями Up и Down. Существующие миграции не редактировать.
- Новый тип упражнения: реализовать `KindHandler`, добавить в `kinds`, включить `Available` в `catalog.go`, добавить проигрыватель и редактор во `features/exercises`.
- Не создавать кнопки для несуществующих функций: показывать честное «Недоступно на этом этапе».
- Не делать push, деплой или миграции на удалённые серверы без отдельного указания.

## Границы текущего этапа (не реализовывать без запроса)

Видеозвонки, камера и микрофон, демонстрация экрана, доска, словарь и переводчик, таймер класса, расписание, ИИ-генерация, остальные типы упражнений, интеграции (Miro, Wordwall, LearningApps, Quizlet…), оплата и тарифы, общая аналитика школы, групповые классы.
