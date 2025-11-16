Вот
полная
текстовая
версия
вашего
ТЗ.

-----

## Техническое

задание:
Парсер
меню
ресторана

### Описание

Разработать
микросервис
на
Go
для
парсинга
меню
из
Google
Таблицы
с
сохранением
в
БД
и
обработкой
через
очередь
сообщений.

### Стек

технологий

  * **Go:**
    1.21+
  * **База
    данных:**
    MongoDB
    6+
  * **Очередь:**
    RabbitMQ
    или
    Redis
    (на
    выбор
    кандидата)
  * **Контейнеризация:**
    Docker,
    Docker
    Compose
  * **API:**
    Google
    Sheets
    API
    v4

### Формат

входных
данных

Google
Таблица
link:

-----

### Основные

задачи

#### 1\.

Парсер
Google
Sheets

  * Подключение
    к
    Google
    Sheets
    API
    через
    Service
    Account
  * Чтение
    всех
    строк
    из
    таблицы
  * Группировка
    данных
  * Преобразование
    в
    целевую
    JSON
    структуру:
    ```json
    {
      "_id": "ObjectId",
      "name": "Restaurant Name",
      "products": [...],
      "attributes_groups": [...],
      "attributes": [...]
    }
    ```
  * Сохранение
    в
    MongoDB
    коллекцию
    `menus`

#### 2\.

Работа
с
очередью

Две
очереди:

  * `menu-parsing`
    —
    задачи
    парсинга
    меню
  * `product-status`
    —
    события
    изменения
    статусов
    продуктов

**Процесс
парсинга:**

1.  АРІ
    получает
    запрос
    \-\>
    создает
    запись
    в
    коллекции
    `parsing_tasks`
    (status:
    `queued`)
2.  Отправляет
    сообщение
    в
    очередь
    `menu-parsing`
3.  Worker
    забирает
    сообщение
    \-\>
    обновляет
    status
    на
    `processing`
4.  Парсит
    Google
    Sheets
    \-\>
    сохраняет
    результат
    в
    `menus`
5.  Обновляет
    status
    на
    `completed`
    или
    `failed`
6.  АСК
    сообщение
    после
    успешной
    обработки

**Обработка
статусов
продуктов:**

1.  АРІ
    получает
    РАТСН
    запрос
    на
    изменение
    статуса
2.  Отправляет
    событие
    в
    очередь
    `product-status`:
    ```json
    {
      "event_type": "product.status_changed",
      "product_id": "1001002",
      "old_status": "available",
      "new_status": "not_available",
      "reason": "out_of_stock",
      "timestamp": "2025-11-14T10:00:00Z",
      "user_id": "admin_123"
    }
    ```
3.  Worker
    обрабатывает
    событие
    \-\>
    обновляет
    статус
    в
    БД
    \-\>
    логирует
    в
    `product_status_audit`

**Типы
событий:**

  * `product.created`
  * `product.updated`
  * `product.status_changed`
  * `product.deleted`

**Требования:**

  * Retry
    механизм
    (3
    попытки
    с
    экспоненциальной
    задержкой)
  * Dead
    Letter
    Queue
    для
    проблемных
    сообщений
  * Graceful
    shutdown
    с
    ожиданием
    завершения
    текущих
    задач
  * Корректное
    закрытие
    соединений

#### 3\.

API
Endpoints

**`POST /api/v1/parse`**

  * **Request:**
    ```json
    {
      "spreadsheet_id": "1ABC...",
      "restaurant_name": "Burger King"
    }
    ```
  * **Response:
    200
    OK**
    ```json
    {
      "task_id": "uuid",
      "status": "queued"
    }
    ```

**`GET /api/v1/parse/{task_id}`**

  * **Response:
    200
    OK**
    ```json
    {
      "task_id": "uuid",
      "status": "completed|processing|failed|queued",
      "menu_id": "ObjectId (если completed)",
      "error": "текст ошибки (если failed)",
      "created_at": "2025-11-14T10:00:00Z",
      "updated_at": "2025-11-14T10:05:00Z"
    }
    ```

**`GET /api/v1/menu/{menu_id}`**

  * **Response:
    200
    OK**
    ```json
    {
      "_id": "68f5c70eeeb0a578f4d5dbd6",
      "name": "Burger King",
      "attributes": [...],
      "attributes_groups": [...],
      "products": [...]
    }
    ```

**`PATCH /api/v1/products/{product_id}/status`**

  * **Request:**
    ```json
    {
      "status": "available|not_available|deleted",
      "reason": "out_of_stock"
    }
    ```
  * **Response:
    202
    Accepted**
    ```json
    {
      "success": true,
      "message": "Status update queued"
    }
    ```

**`GET /api/v1/health`**

  * **Response:
    200
    OK**
    ```json
    {
      "status": "healthy",
      "timestamp": "2025-11-14T10:00:00Z",
      "services": {
        "database": "ok",
        "queue": "ok"
      }
    }
    ```

#### 4\.

Структура
MongoDB

**Коллекция
`menus`:**

```javascript
{
  _id: ObjectId,
  name: String,
  restaurant_id: String, // для поддержки нескольких ресторанов
  products: Array,
  attributes_groups: Array,
  attributes: Array,
  created_at: ISODate,
  updated_at: ISODate
}
```

**Коллекция
`parsing_tasks`:**

```javascript
{
  _id: UUID,
  status: String, // queued, processing, completed, failed
  spreadsheet_id: String,
  restaurant_name: String,
  menu_id: ObjectId,
  error_message: String,
  retry_count: Number,
  created_at: ISODate,
  updated_at: ISODate
}
```

**Коллекция
`product_status_audit`:**

```javascript
{
  _id: ObjectId,
  product_id: String,
  event_type: String,
  old_status: String,
  new_status: String,
  reason: String,
  user_id: String,
  timestamp: ISODate
}
```

**Индексы:**

  * `menus`:
    `restaurant_id`,
    `products.ext_id`
  * `parsing_tasks`:
    `_id`,
    `status`,
    `created_at`
  * `product_status_audit`:
    `product_id`,
    `timestamp`

#### 5\.

Docker
Compose

**Сервисы:**

  * ## `api`
    HTTP
    API
    сервер
    (порт
    8080\)
  * ## `worker`
    Worker
    для
    обработки
    очередей
  * ## `mongodb`
    База
    данных
    (порт
    27017\)
  * ## `rabbitmq` или `redis`
    Очередь
    сообщений

**Требования:**

  * Все
    сервисы
    в
    одной
    Docker
    сети
    `menu-parser-network`
  * Healthchecks
    для
    всех
    сервисов
  * `depends_on`
    c
    `condition: service_healthy`
  * Named
    volumes
    для
    персистентности
    данных
    MongoDB
  * Переменные
    окружения
    через
    `env`
    файл
  * Multi-stage
    Dockerfile
    для
    минимизации
    размера
    образа

#### 6\.

Технические
требования

**MongoDB:**

  * Использовать
    официальный
    драйвер
    go.mongodb.org/mongo-driver
  * # Connection pool: MaxPoolSize
    # 100 , MinPoolSize
    10
  * Context
    с
    таймаутами
    для
    всех
    операций
    (5-30
    секунд)
  * Транзакции
    для
    критичных
    операций
  * Defer
    для
    закрытия
    курсоров
  * Graceful
    disconnect
    при
    shutdown