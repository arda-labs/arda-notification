# arda-notification

Real-time notification service cho arda.io.vn platform, viết bằng **Go**. Implement mô hình **fan-out on write** (inbox per user) — mỗi user có inbox riêng, idempotent với Kafka at-least-once delivery.

## Stack

| Layer          | Technology                                            |
| -------------- | ----------------------------------------------------- |
| HTTP Framework | [Echo v4](https://echo.labstack.com/)                 |
| Kafka Consumer | [franz-go](https://github.com/twmb/franz-go)          |
| Database       | PostgreSQL via [pgx/v5](https://github.com/jackc/pgx) |
| Config         | [Viper](https://github.com/spf13/viper)               |
| Logging        | [zerolog](https://github.com/rs/zerolog)              |
| Auth           | Keycloak JWT (JWKS) + Admin REST API                  |

## Architecture

```
Kafka Topics
 ├── tenant-events         → Tenant provisioned/deleted → fan-out tới PLATFORM_ADMIN role
 ├── bpm-events            → Task assigned/completed    → 1 user (assignee)
 ├── crm-events            → Lead/deal updates          → 1 user (owner)
 ├── iam-events            → Login/password alerts      → 1 user (subject)
 └── notification-commands → Direct push, hỗ trợ 4 scope (USER/TENANT/PLATFORM/ROLE)
          ↓
 Kafka Consumer (franz-go)
   └── service.Fanout(FanoutInput)
         ├── ScopeUser     → insert 1 row trực tiếp
         ├── ScopeTenant   → query Keycloak → batch insert N rows
         ├── ScopeRole     → query Keycloak → batch insert N rows
         └── ScopePlatform → query Keycloak (tất cả realms) → batch insert
                  ↓
         PostgreSQL — notifications table
         (mỗi row = 1 user_id cụ thể)
                  ↓
 HTTP Transport (Echo)
   ├── REST API    → Pull mode (list, mark read, delete)
   └── SSE Stream  → Real-time push khi có notification mới
```

### Tại sao fan-out on write?

> Mô hình được dùng bởi Slack, Linear, Notion, Jira:

- **Query đơn giản**: `WHERE user_id = ?` — không JOIN, không subquery
- **`is_read` per-user tự nhiên** — không cần bảng phụ
- **Dễ paginate/sort/filter** theo từng user
- **PostgreSQL** với composite index handle 10M+ rows bình thường
- Schema `notifications` không bao giờ chứa magic string — luôn là UUID cụ thể

---

## API Endpoints

| Method   | Path                                              | Mô tả                          |
| -------- | ------------------------------------------------- | ------------------------------ |
| `GET`    | `/api/notification/v1/notifications`              | List notifications (paginated) |
| `GET`    | `/api/notification/v1/notifications/unread-count` | Badge count                    |
| `PATCH`  | `/api/notification/v1/notifications/:id/read`     | Mark single read               |
| `POST`   | `/api/notification/v1/notifications/read-all`     | Mark all read                  |
| `DELETE` | `/api/notification/v1/notifications/:id`          | Delete                         |
| `GET`    | `/api/notification/v1/notifications/stream`       | **SSE stream**                 |
| `GET`    | `/health`                                         | Health check                   |

### Headers Required

```
Authorization: Bearer <keycloak-jwt>
X-Tenant-Key: <tenant-key>
```

---

## SSE Integration (Frontend)

```typescript
const es = new EventSource("/api/notification/v1/notifications/stream", {
  headers: {
    Authorization: `Bearer ${token}`,
    "X-Tenant-Key": tenantKey,
  },
});

es.addEventListener("notification", (e) => {
  const notification = JSON.parse(e.data);
  // Update badge count, show toast, etc.
});
```

---

## Kafka — TargetScope (Fan-out Model)

### notification-commands format

```json
{
  "commandId": "unique-idempotency-key",
  "tenantKey": "acme-corp",
  "targetScope": "TENANT",
  "targetId": "acme-corp",
  "type": "SYSTEM",
  "title": "Maintenance tonight",
  "body": "System will be down 2-4 AM",
  "metadata": {}
}
```

| `targetScope` | `targetId`      | Fan-out                                   | Ví dụ                        |
| ------------- | --------------- | ----------------------------------------- | ---------------------------- |
| `USER`        | Keycloak userID | 1 row, trực tiếp                          | Task assigned, IAM alert     |
| `TENANT`      | tenantKey       | N rows — tất cả user trong tenant         | Admin broadcast, quota alert |
| `PLATFORM`    | _(bỏ trống)_    | N rows — tất cả active user trên platform | System maintenance           |
| `ROLE`        | roleName        | N rows — user có role đó trong tenant     | Alert chỉ cho ADMIN          |

### Kafka Event Envelope (từ Java services)

```json
{
  "eventType": "TASK_ASSIGNED",
  "eventId": "uuid-for-idempotency",
  "tenantKey": "acme-corp",
  "payload": { "...": "domain-specific data" }
}
```

### Supported event types

| Topic           | eventType             | TargetScope | Ghi chú                 |
| --------------- | --------------------- | ----------- | ----------------------- |
| `tenant-events` | `TENANT_CREATED`      | ROLE        | → role `PLATFORM_ADMIN` |
| `tenant-events` | `TENANT_DELETED`      | ROLE        | → role `PLATFORM_ADMIN` |
| `bpm-events`    | `TASK_ASSIGNED`       | USER        | → payload.assigneeId    |
| `bpm-events`    | `TASK_COMPLETED`      | USER        | → payload.assigneeId    |
| `bpm-events`    | `APPROVAL_REQUIRED`   | USER        | → payload.assigneeId    |
| `crm-events`    | `LEAD_STATUS_CHANGED` | USER        | → payload.ownerId       |
| `crm-events`    | `DEAL_UPDATED`        | USER        | → payload.ownerId       |
| `iam-events`    | `LOGIN_NEW_DEVICE`    | USER        | → payload.userId        |
| `iam-events`    | `PASSWORD_CHANGED`    | USER        | → payload.userId        |

---

## Environment Variables

| Variable                        | Default                     | Mô tả                                   |
| ------------------------------- | --------------------------- | --------------------------------------- |
| `PORT`                          | `8090`                      | HTTP port                               |
| `DB_HOST`                       | `localhost`                 | PostgreSQL host                         |
| `DB_PORT`                       | `5432`                      | PostgreSQL port                         |
| `DB_NAME`                       | `arda_notification`         | Database name                           |
| `DB_USER`                       | `postgres`                  | DB user                                 |
| `DB_PASSWORD`                   | `password`                  | DB password                             |
| `KAFKA_BROKERS`                 | `localhost:9092`            | Kafka brokers (comma-separated)         |
| `KEYCLOAK_URL`                  | `http://localhost:8081`     | Keycloak base URL                       |
| `KEYCLOAK_ADMIN_REALM`          | `master`                    | Realm dùng để lấy admin token           |
| `KEYCLOAK_ADMIN_CLIENT_ID`      | `arda-notification-service` | Client ID cho Keycloak Admin API        |
| `KEYCLOAK_ADMIN_CLIENT_SECRET`  | _(required)_                | Client secret — **phải set trong prod** |
| `ARDA_NOTIF_TTL_RETENTION_DAYS` | `30`                        | Notification retention in days          |

---

## Development

```bash
# Run locally
go run ./cmd/server

# Build binary
go build -o arda-notification ./cmd/server

# Build Docker image
docker build -t arda-notification .
```

## Database Setup

```bash
psql -h localhost -U postgres -d arda_notification \
  -f migrations/001_create_notifications_table.sql
```

---

## Cần làm thêm (Checklist)

### 1. Tạo Keycloak client `arda-notification-service`

Chạy script có sẵn — idempotent, safe to re-run:

```bash
# Keycloak phải đang chạy trước (docker compose up arda-keycloak)
bash arda-infra-config/scripts/setup-keycloak-notification-client.sh
```

Script sẽ tự động:

1. Đăng nhập Keycloak bằng admin credentials
2. Tạo client `arda-notification-service` (confidential, service accounts enabled)
3. Gán role `view-users` và `query-realms` từ client `realm-management`
4. In ra **Client Secret** để điền vào `docker-compose.yml`

> **Override** URL/credentials nếu khác default:
>
> ```bash
> KEYCLOAK_URL=http://localhost:8081 \
> KEYCLOAK_ADMIN=admin \
> KEYCLOAK_ADMIN_PASSWORD=admin \
> bash scripts/setup-keycloak-notification-client.sh
> ```

---

### 2. Cập nhật `KEYCLOAK_ADMIN_CLIENT_SECRET` trong docker-compose

File [`docker-compose.yml`](../arda-infra-config/docker-compose/docker-compose.yml) đã có placeholder:

```yaml
# arda-infra-config/docker-compose/docker-compose.yml
arda-notification:
  environment:
    - KEYCLOAK_ADMIN_REALM=master
    - KEYCLOAK_ADMIN_CLIENT_ID=arda-notification-service
    - KEYCLOAK_ADMIN_CLIENT_SECRET=change-me-run-setup-script # ← thay bằng secret thật
```

Sau khi chạy script ở bước 1, copy secret và thay vào đây:

```bash
# Chạy lại script để xem secret (nếu quên)
bash arda-infra-config/scripts/setup-keycloak-notification-client.sh
```

---

### 3. Fix môi trường `go mod tidy` (proxy/internet)

Hiện tại `go mod tidy` bị lỗi do không resolve được revision của `golang.org/x/exp`:

```
golang.org/x/exp@v0.0.0-20241217172543-b2144cdd0a42: invalid version: unknown revision
```

**Cách fix (chọn 1 trong 3):**

```bash
# Option A: Dùng proxy của Google (nếu có internet)
$env:GOPROXY = "https://proxy.golang.org,direct"
go mod tidy

# Option B: Dùng Athens (self-hosted Go module proxy)
$env:GOPROXY = "http://athens:3000,direct"
go mod tidy

# Option C: Bỏ qua sum check
$env:GONOSUMCHECK = "*"
$env:GOFLAGS = "-mod=mod"
go mod tidy
```

Sau khi tidy thành công, commit cả `go.mod` và `go.sum`.

### 1. Tạo Keycloak client `arda-notification-service`

Service cần gọi Keycloak Admin REST API để resolve fan-out targets. Cần tạo client **service account** trong realm `master`:

```
Keycloak Admin Console
  → Realm: master
  → Clients → Create client
      Client ID:              arda-notification-service
      Client authentication:  ON  (confidential)
      Service accounts roles: ON
      Standard flow:          OFF

  → Tab "Service account roles"
      → Assign role: "view-users" (realm-management client)
      → Assign role: "query-realms" (master realm — cần để list all realms cho PLATFORM scope)
```

> **Lưu ý**: Role `view-users` thuộc client `realm-management`, không phải realm role.  
> Vào: Clients → `realm-management` → Roles → `view-users` → Assign to service account.

Sau khi tạo client, copy **Client Secret** ở tab "Credentials".

---

### 2. Set `KEYCLOAK_ADMIN_CLIENT_SECRET` trong docker-compose

Mở file `docker-compose.yml` (hoặc `docker-compose.override.yml`) và thêm:

```yaml
services:
  arda-notification:
    environment:
      KEYCLOAK_URL: "http://keycloak:8080"
      KEYCLOAK_ADMIN_REALM: "master"
      KEYCLOAK_ADMIN_CLIENT_ID: "arda-notification-service"
      KEYCLOAK_ADMIN_CLIENT_SECRET: "<secret-from-keycloak>"
```

> **Không commit secret vào git.** Dùng `.env` file hoặc Docker secrets trong production.

```yaml
# Khuyến nghị dùng .env
services:
  arda-notification:
    env_file:
      - .env.notification
```

---

### 3. Fix môi trường `go mod tidy` (proxy/internet)

Hiện tại `go mod tidy` bị lỗi do không resolve được revision của `golang.org/x/exp`:

```
golang.org/x/exp@v0.0.0-20241217172543-b2144cdd0a42: invalid version: unknown revision
```

**Nguyên nhân**: Môi trường dev không có internet trực tiếp hoặc GOPROXY chưa cấu hình đúng.

**Cách fix (chọn 1 trong 3):**

```bash
# Option A: Dùng proxy của Google (nếu có internet)
$env:GOPROXY = "https://proxy.golang.org,direct"
go mod tidy

# Option B: Dùng Athens (self-hosted Go module proxy)
$env:GOPROXY = "http://athens:3000,direct"
go mod tidy

# Option C: Dùng GONOSUMCHECK nếu dùng private proxy
$env:GONOSUMCHECK = "*"
$env:GOFLAGS = "-mod=mod"
go mod tidy
```

Sau khi tidy thành công, commit cả `go.mod` và `go.sum`.
