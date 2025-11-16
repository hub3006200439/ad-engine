### Используется:
- **Go 1.25**
- Хранилище: **PostgreSQL**
- Сетевой стек: **fasthttp** + **fasthttp/router**
- Логирование: **mtlog** (консоль + файл, структурированные логи)
- Идентификаторы: **UUID** для всех ключевых сущностей
- Формат конфигурации: **TOML**

Цель:  
Реализовать backend-сервис, который:

1. Подбирает релевантный рекламный ролик для просмотра видео.
2. Учитывает события: **impression / click / view**.
3. Корректно и конкурентно-безопасно списывает бюджет кампании (**CPM / CPC**).
4. Возвращает базовую статистику по кампании за период.

---

## 1. MVP

### User Stories → Реализация

#### Зритель

> Зритель получает релевантный ролик с учётом контекста и ограничения повторов; если подходящего нет — no-fill.

Реализация:

- HTTP-эндпоинт: `POST /ad/request`
- Контекст запроса:
  - `user_id` (UUID зрителя)
  - `session_id` (UUID сессии)
  - язык (`lang`)
  - страна (`country`)
  - категория (`category`)
  - placement (`placement` — preroll/midroll/postroll)
  - `request_id` (опциональный идемпотентный ключ)
- Алгоритм:
  1. По таргетингу выбираются кандидаты из БД (`campaigns` + `creatives`).
  2. Кандидаты ранжируются по **ставке + совпадениям таргета**.
  3. Лочится выбранная кампания (`FOR UPDATE`).
  4. Проверяется **frequency cap** по `(user_id, campaign_id, day)`.
  5. Проверяются **бюджетные лимиты**.
  6. При успехе создаётся `impression` и (для CPM) списывается бюджет.
  7. Возвращается креатив; при невозможности → `204 No Content`.

#### Рекламодатель

> Рекламодатель видит честный расход бюджета: один раз по модели оплаты, соблюдение дневных/общих лимитов.

Реализация:

- Модели:
  - **CPM** — списание при показе (`impression`).
  - **CPC** — списание при клике (`click`).
- Лимиты:
  - `budget_total` (общий бюджет кампании).
  - `budget_daily` (дневной лимит).
- Списания:
  - При CPM: `cost = bid_micros / 1000` за показ; минимальный `cost >= 1`.
  - При CPC: `cost = bid_micros` при **первом** клике на impression.
- Все списания фиксируются в таблице **`spend_events`** с типами:
  - `CPM_IMPRESSION`
  - `CPC_CLICK`
- Защита от гонок:
  - `SELECT ... FOR UPDATE` кампании.
  - Изменения бюджетов и запись `spend_events` в одной транзакции.

#### Система / оператор

> Система фиксирует события и отдаёт статистику; оператор может получить сводку по кампании за период.

Реализация:

- События:
  - `impressions` — в `RequestAd`.
  - `clicks` — в `TrackClick`.
  - `views` — в `TrackView`.
- Статистика:
  - Эндпоинт: `GET /stats/overview?from=&to=&campaign_id=`.
  - Возвращает:
    - `impressions`
    - `clicks`
    - `views`
    - `CTR`
    - `spend_micros` (по `spend_events`).

---

## 2. Общая архитектура и структура проекта

Слои:

- **HTTP-слой** (`internal/http`):
  - парсит запросы,
  - валидирует,
  - вызывает usecase-сервисы и маршалит ответы.
- **Usecase-слой** (`internal/usecase`):
  - бизнес-логика:
    - подбор объявления,
    - статистика,
    - частотные лимиты.
- **Repository-слой** (`internal/repo/postgres`):
  - конкретная реализация работы с Postgres.
- **Infrastructure**:
  - `internal/logger` — mtlog-логирование.
  - `internal/config` — загрузка TOML-конфигурации.
  - `cmd/adserver/main.go` — сборка зависимостей, запуск сервера.

## 3. Схема данных (Postgres, UUID)

Ключевые таблицы:

- `campaigns`  
  - `id UUID PK`
  - `name TEXT`
  - `pricing_model TEXT` (`CPM` / `CPC`)
  - `bid_micros BIGINT`
  - `budget_total BIGINT`
  - `budget_daily BIGINT`
  - `spent_total BIGINT`
  - `spent_today BIGINT`
  - `spent_today_date DATE`
  - `frequency_cap INT`
  - `targeting_json JSONB`:
    - `languages`, `countries`, `categories`, `placements`
  - `active BOOL`
  - `start_at`, `end_at`

- `creatives`
  - `id UUID PK`
  - `campaign_id UUID FK → campaigns`
  - `duration_sec INT`
  - `video_url TEXT`
  - `landing_url TEXT`
  - `lang TEXT`
  - `placement TEXT`
  - `category TEXT`

- `impressions`
  - `id UUID PK`
  - `token TEXT UNIQUE`
  - `request_id TEXT NULL` (для идемпотентности `/ad/request`)
  - `campaign_id UUID`
  - `creative_id UUID`
  - `user_id UUID`
  - `session_id UUID`
  - `created_at TIMESTAMPTZ DEFAULT now()`

- `clicks`
  - `id UUID PK`
  - `impression_id UUID UNIQUE` (идемпотентность клика)
  - `token TEXT UNIQUE`
  - `created_at TIMESTAMPTZ DEFAULT now()`

- `views`
  - `id UUID PK`
  - `impression_id UUID UNIQUE` (идемпотентность просмотра)
  - `created_at TIMESTAMPTZ DEFAULT now()`

- `user_campaign_daily`
  - `user_id UUID`
  - `campaign_id UUID`
  - `day DATE`
  - `impressions INT`
  - `PRIMARY KEY (user_id, campaign_id, day)`

- `spend_events`
  - `id UUID PK`
  - `campaign_id UUID`
  - `event_type TEXT` (`CPM_IMPRESSION` / `CPC_CLICK`)
  - `amount_micros BIGINT`
  - `created_at TIMESTAMPTZ DEFAULT now()`

---

## 4. HTTP API cURL-примеры
### [можно посмотреть в Swagger:](swagger.json)

### `POST /ad/request`

Запрос:

```json
{
  "user_id":   "3c8ce0e0-ff1e-4d45-9be7-1a3b35d1a123",
  "session_id":"c0b9036b-6710-4d07-8771-48d16f6f9baf",
  "lang":      "en",
  "country":   "US",
  "category":  "tech",
  "placement": "preroll",
  "request_id":"d5b2e9cb-74b6-4d13-97e0-b5f3f9c58b0a"
}
```

Ответ 200:

```json
{
  "token":       "8b9d9c0f-96b6-43d2-8d2d-a0a3fae2d132",
  "creative_id": "5e0af668-6b46-4f45-9fbd-9896505b78c4",
  "duration":    15,
  "video_url":   "https://cdn.example.com/video_cfdc2f6b.mp4",
  "click_url":   "/ad/click/8b9d9c0f-96b6-43d2-8d2d-a0a3fae2d132",
  "view_url":    "/ad/view/8b9d9c0f-96b6-43d2-8d2d-a0a3fae2d132"
}
```

Ответ 204:  
нет тела, если подходящего креатива нет (no-fill).

**cURL-пример:**

```bash
curl -X POST "http://localhost:8443/ad/request" -H "Content-Type: application/json" -d '{
    "user_id":   "3c8ce0e0-ff1e-4d45-9be7-1a3b35d1a123",
    "session_id":"c0b9036b-6710-4d07-8771-48d16f6f9baf",
    "lang":      "en",
    "country":   "US",
    "category":  "tech",
    "placement": "preroll",
    "request_id":"d5b2e9cb-74b6-4d13-97e0-b5f3f9c58b0a"
  }' -v
```
---

### `GET /ad/click/{token}`

- Регистрирует клик по impression.
- Для CPC-кампании выполняет списание бюджета.
- Идемпотентность: вторые и последующие клики на тот же impression **не списывают** деньги.

Пример запроса:

```bash
curl -X GET "http://localhost:8443/ad/click/8b9d9c0f-96b6-43d2-8d2d-a0a3fae2d132"   -v -L
```

- Первый клик:
  - `302 Found`, заголовок `Location: https://example.com/landing`.
- Повторный клик:
  - тоже `302`, но **без повторного списания** в биллинге.

---

### `GET /ad/view/{token}`

- Регистрирует факт просмотра ролика (`view`).
- Идемпотентность за счёт `UNIQUE(impression_id)` в `views`.

Пример:

```bash
curl -X GET "http://localhost:8443/ad/view/8b9d9c0f-96b6-43d2-8d2d-a0a3fae2d132" -v
```

Ответы:

- `204 No Content` — ok/повторный вызов.
- `404 Not Found` — если impression не найден.

---

### `GET /stats/overview?from=&to=&campaign_id=`

Пример:

```bash
curl "http://localhost:8443/stats/overview?from=2025-11-01T00:00:00Z&to=2025-11-14T00:00:00Z&campaign_id=5e0af668-6b46-4f45-9fbd-9896505b78c4"
```

Ответ:

```json
{
  "campaign_id":  "5e0af668-6b46-4f45-9fbd-9896505b78c4",
  "from":         "2025-11-01T00:00:00Z",
  "to":           "2025-11-14T00:00:00Z",
  "impressions":  12345,
  "clicks":       678,
  "views":        500,
  "ctr":          0.0549,
  "spend_micros": 123456789
}
```

Если `campaign_id` не указан — статистика по всем кампаниям.

---

## 5. Алгоритм подбора ролика (RequestAd)

Упрощённо:

1. **Идемпотентность по `request_id`**
   - Если `request_id` задан → ищем `impression` с таким `request_id`:
     - если найден → возвращаем тот же креатив и токен;
     - если нет — продолжаем.

2. **Начинаем транзакцию**

3. **Выбор кандидатов (таргетинг)**

   SQL (`selectCandidatesSQL`):

   - фильтруем кампании по:
     - `active = true`
     - `start_at <= now() <= end_at`
     - `budget_total / budget_daily` — ещё не исчерпаны
   - фильтруем таргетинг по `targeting_json`:
     - языки, страны, категории, placements
   - JOIN креативов `creatives` (одна кампания → несколько креативов).
   - `LIMIT 100` для защиты от слишком широких кампаний.

4. **Ранжирование кандидатов**

   Функция `pickBestCandidate`:

   - базовый скор = `bid_micros`;
   - бонусы:
     - совпадение языка -> `+ bid / 2`
     - совпадение placement -> `+ bid / 3`
     - совпадение категории -> `+ bid / 4`
   - выбираем кандидата с максимальным скором.

5. **Лочим кампанию и читаем бюджеты**

   ```sql
   SELECT ...
   FROM campaigns
   WHERE id = $1
   FOR UPDATE
   ```

   Получаем:

   - pricing_model
   - bid_micros
   - budget_total / budget_daily
   - spent_total / spent_today + spent_today_date
   - frequency_cap

6. **Проверяем freq cap**

   Через `FrequencyService`:

   ```go
   allowed, err := freq.Allowed(ctx, userID, campaignID)
   if !allowed {
       // no-fill
   }
   ```

   Внутри:

   - читаем `campaigns.frequency_cap`;
   - читаем `user_campaign_daily` для `(user_id, campaign_id, today)`;
   - сравниваем `impressions < frequency_cap`.

7. **Списываем CPM-бюджет (если нужно)**

   - Если `pricing_model == CPM`:
     - `costMicros = bid_micros / 1000` (но как минимум 1).
     - проверяем:
       - `spent_total + costMicros <= budget_total` (если задано),
       - `spent_today + costMicros <= budget_daily` (если задано).
     - обновляем `spent_total`, `spent_today`, `spent_today_date`.
     - добавляем запись в `spend_events (CPM_IMPRESSION)`.

   - Если `pricing_model == CPC`:
     - списание откладывается до клика;
     - `costMicros = 0` в этом шаге.

8. **Создаём impression**

   - Генерируем `token` (UUID).
   - Пишем запись в `impressions`:
     - `id`, `token`, `request_id`, `campaign_id`, `creative_id`, `user_id`, `session_id`.

9. **Регистрация freq cap**

    - Уже после `Commit`:
      ```go
      freq.Register(ctx, userID, campaignID)
      ```
    - Внутри:
      - `INSERT INTO user_campaign_daily (...) ON CONFLICT DO UPDATE ...`.

10. **Возврат ответа**

    - Формируем `AdResponse` с:
      - `token`
      - `creative_id`
      - `duration`
      - `video_url`
      - `click_url` (`/ad/click/{token}`)
      - `view_url` (`/ad/view/{token}`)

---

## 6. Алгоритм биллинга (CPM/CPC)

### CPM: при показе

- **Где:** `RequestAd`, внутри транзакции.
- **Когда:** сразу после выбора кампании, до `impression`.
- **Сколько:** `bid_micros / 1000` за показ.
- **Гарантии:**
  - `FOR UPDATE` кампании + проверка лимитов.
  - Запись `spend_events` в той же транзакции.
  - Если лимит/бюджет не хватает → no-fill, ничего не списывается.

### CPC: при клике

- **Где:** `TrackClick`.
- **Шаги:**
  1. Начинаем транзакцию.
  2. Ищем `impression` по `token` и JOIN-им `campaign` + `creative`:
     - `FOR UPDATE OF campaigns` — лочим кампанию.
  3. Пытаемся вставить запись в `clicks`:
     ```sql
     INSERT INTO clicks (impression_id, token)
     VALUES ($1, $2)
     ON CONFLICT (impression_id) DO NOTHING
     ```
     - Если `RowsAffected == 0` → клик уже был, **деньги второй раз не списываются**.
  4. Если это **первый** клик:
     - для `pricing_model == CPC` списываем `bid_micros`:
       - проверяем лимиты;
       - обновляем `spent_total`, `spent_today`;
       - пишем `spend_events (CPC_CLICK)`.
  5. Коммит транзакции.
  6. Возвращаем `landing_url` для редиректа.

---

## 7. Frequency capping: architecture & implementation

### Цель

Ограничить количество показов кампании на пользователя за день:
не более `frequency_cap` показов на `(user_id, campaign_id, day)`.

### Схема хранения

```sql
CREATE TABLE user_campaign_daily (
    user_id     UUID NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    day         DATE NOT NULL,
    impressions INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, campaign_id, day)
);
```

Поле `campaigns.frequency_cap` задаёт максимальное количество показов
кампании на одного пользователя в день. Значение `0` трактуется как «без ограничений».

### Архитектура

Логика freq cap вынесена в отдельный usecase-слой:

- `internal/usecase/frequency.go`
  - `FrequencyRepository` — интерфейс над хранилищем freq cap.
  - `FrequencyService` — бизнес-операции:
    - `Allowed(ctx, userID, campaignID) (bool, error)`
    - `Register(ctx, userID, campaignID) error`.

- `internal/repo/postgres/freq_repo.go`
  - `FreqRepository` — реализация `FrequencyRepository` для Postgres:
    - читает `campaigns.frequency_cap`;
    - читает/обновляет `user_campaign_daily` (через `INSERT ... ON CONFLICT`).

- `internal/repo/postgres/ad_repo.go`
  - использует `FrequencyService` на hot path:
    - перед показом:
      ```go
      allowed, err := freq.Allowed(ctx, userID, campaignID)
      if !allowed {
          // no-fill
      }
      ```
    - после успешного показа:
      ```go
      if err := freq.Register(ctx, userID, campaignID); err != nil {
          // log / fail
      }
      ```

### Алгоритм

1. `POST /ad/request`:
   - после выбора кампании и её блокировки:
     - `FrequencyService.Allowed`:
       - читает `frequency_cap`;
       - считает `impressions` за сегодня;
       - сравнивает `< frequency_cap`.
   - если лимит превышен → **no-fill**.

2. После создания `impression` и списания CPM:
   - `FrequencyService.Register`:
     - увеличивает `impressions` в `user_campaign_daily`.

### Причины такого дизайна

- **Одна точка правды** для freq cap — `FrequencyService + FreqRepository`.
- Логика частотных ограничений не размазана по SQL в разных местах.
- Легко заменить реализацию на Redis / другой стор, не меняя usecase-слой.

---

## 8. Идемпотентность

### `/ad/request` — по `request_id`

- При наличии `request_id`:
  - сначала ищем `impressions.request_id = $1`.
  - если найдено → возвращаем тот же токен/креатив.
  - если нет → создаём новый impression и записываем этот `request_id`.

### `/ad/click/{token}` — по `impression_id`

- `clicks.impression_id` имеет `UNIQUE` constraint.
- При вставке:
  ```sql
  INSERT INTO clicks (impression_id, token)
  VALUES ($1, $2)
  ON CONFLICT (impression_id) DO NOTHING
  ```
- Если `RowsAffected == 0` → клик уже был, **деньги второй раз не списываются**.

### `/ad/view/{token}`

- `views.impression_id` также уникален.
- Вставка:
  ```sql
  INSERT INTO views (impression_id)
  SELECT i.id FROM impressions i WHERE i.token = $1
  ON CONFLICT (impression_id) DO NOTHING
  ```
- Повторные запросы `/ad/view/{token}` не создают дубликатов.

---

## 9. Статистика (StatsService + StatsRepository)

Usecase: `internal/usecase/stats_service.go`  
Repo: `internal/repo/postgres/stats_repo.go`

Алгоритм:

1. Парсим `from`, `to`, `campaign_id` из query-параметров.
2. В `StatsRepository`:
   - считаем `impressions`:
     ```sql
     SELECT COUNT(*) FROM impressions WHERE campaign_id = $1 AND created_at BETWEEN $2 AND $3
     ```
   - `clicks` из `clicks`.
   - `views` из `views`.
   - `spend_micros` как сумму `amount_micros` из `spend_events`.
3. Считаем `CTR = clicks / impressions` (0, если показов нет).
4. Возвращаем JSON с агрегатами.