# API 1С:ГРМ Partner API v2

## Базовая информация

- **Базовый URL:** `https://service-api.1capp.com/partner-api/v2`
- **Auth:** Bearer JWT токен
- **Формат:** JSON (OpenAPI 3.0.1)

---

## Авторизация

```http
GET /applications
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...
```

---

## Приложения (Applications)

### Список приложений
```
GET /applications?page=0&size=20
```

**Ответ:**
```json
[
  {
    "id": "uuid",
    "configurationId": "uuid",
    "configurationName": "1С:Розница 8.3",
    "configurationVersionId": "uuid",
    "platformVersionId": "uuid",
    "platformVersion": "8.3.25.1445",
    "name": "1С:Розница 8.3",
    "status": "running",
    "url": "app123456.1capp.net/...",
    "licenseCount": 3,
    "customerId": "uuid",
    "tariff": "BUSINESS",
    "billingState": "PAY",
    "comment": null,
    "diskQuota": 5368709120
  }
]
```

**Статусы:** `running`, `stopped`, `deleting`

### Детали приложения
```
GET /applications/{applicationId}
```

**Ответ:** тот же формат, что и в списке

### Управление
```
POST /applications/{id}/action/start
POST /applications/{id}/action/restart
POST /applications/{id}/action/stop
POST /applications/{id}/action/cancelDelete
PUT /applications/{id}/comment
```

---

## Бэкапы (Backups)

### Список бэкапов
```
GET /applications/{applicationId}/backups
```

**Ответ:**
```json
[
  {
    "id": "ff808081a138b74901a13cc339e31cf1",
    "date": "2026-10-14T23:37:49.577+00:00",
    "size": 6154.65,
    "backupType": "regular",
    "link": "https://subscriptions.hb.bizmrg.com/...",
    "status": "ready"
  }
]
```

**Поля:**
- `id` — ID бэкапа
- `date` — дата создания (ISO 8601)
- `size` — размер в MB
- `backupType` — `regular`, `technical`, `manual`
- `link` — прямая ссылка для скачивания (S3 presigned URL)
- `status` — `ready`, `creating`, `failed`

### Создание бэкапа
```
POST /applications/{id}/backups
Content-Type: application/json

{
  "type": "full"
}
```

### Восстановление
```
POST /applications/{id}/backups/{backupId}/restore
```

### Удаление
```
DELETE /applications/{id}/backups/{backupId}
```

---

## Номенклатура конфигураций

```
GET /configurations/nomenclatures
```

---

## Клиенты (Customers)

```
GET /customers
GET /customers/{customerId}
GET /customers/findByInn?inn=1234567890
GET /customers/findByEmail?email=test@example.com
```

---

## Заказы (Orders)

```
GET /orders
GET /orders/status/{orderId}
GET /orders/services
GET /orders/services/{serviceKitId}
```

---

## Версии платформ

```
GET /applications/platformVersions
GET /applications/platformVersions/{versionId}
```

---

## Обмены (Exchanges)

```
POST /applications/{id}/exchanges/upload
POST /applications/{id}/exchanges/install
POST /applications/{id}/exchanges/dump
GET /applications/{id}/exchanges
DELETE /applications/{id}/exchanges/{exchangeId}
```

---

## Техжурнал (Techlog)

```
POST /applications/{id}/techlog/upload
POST /applications/{id}/techlog/on
POST /applications/{id}/techlog/off
GET /applications/{id}/techlog/events
```

---

## Ресурсы приложения

```
GET /applications/{id}/resources
```

Возвращает: CPU, RAM, disk usage

---

## Коды ответов

| Код | Описание |
|---|---|
| 200 | OK |
| 201 | Created |
| 204 | No Content |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 429 | Too Many Requests |
| 500 | Internal Server Error |

---

## Ошибки

Формат ошибки:
```json
{
  "timestamp": "2026-10-16T08:35:40Z",
  "message": "Invalid token",
  "errorCode": null
}
```

---

## Примечания

1. **Ссылки на бэкапы** имеют ограниченный срок жизни (обычно 4 часа)
2. **Лимиты API:** ~100 запросов в минуту
3. **Размер бэкапов:** от 100 MB до нескольких GB
4. **Скорость скачивания:** зависит от тарифа 1С:ГРМ
5. **JWT токен** требует обновления (срок жизни ~24 часа)
