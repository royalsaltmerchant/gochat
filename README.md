# Parch

**The user-hosted messenger and voice chat application.**

Parch is a real-time chat and voice platform built around decentralization and simplicity. It allows users to run their own servers (hosts), connect through a public relay, and communicate securely via self-hosted infrastructure.

---

## Architecture

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Desktop Client │  │   Web Client    │  │    Call App     │
│     (Wails)     │  │  (Vite/Browser) │  │  (React/Vite)   │
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                          │
             ┌────────────▼────────────┐
             │      Relay Server       │
             │         (Go)            │
             └────────────┬────────────┘
                          │
           ┌──────────────┼──────────────┐
           │              │              │
  ┌────────▼────────┐ ┌───▼───┐ ┌────────▼────────┐
  │   Host Client   │ │  SFU  │ │   TURN Server   │
  │  (Go + SQLite)  │ │(Pion) │ │    (coturn)     │
  └─────────────────┘ └───────┘ └─────────────────┘
```

### Components

| Component | Description |
|-----------|-------------|
| **Relay Server** | Central Go service for host/user registration, WebSocket signaling, and connection relay |
| **Host Client** | Desktop app that stores chat data locally (SQLite). Manages spaces, channels, and messages |
| **Desktop Client** | Wails-based desktop chat application for end users |
| **Web Client** | Browser build of the same frontend client code used by the desktop app |
| **Call App** | React web app for standalone video/voice calls (no authentication required) |
| **SFU** | Pion-based Selective Forwarding Unit for routing voice/video streams |
| **TURN Server** | coturn server for NAT traversal in WebRTC connections |

---

## Environment Variables

Create a `.env` file in `relay_server/`:

```bash
# Server
RELAY_PORT=8000
HOST_DB_FILE=./relay.db

# Authentication
JWT_SECRET=your-jwt-secret-key

# TURN Server (coturn)
TURN_URL=turn:your-turn-server:3478
TURN_SECRET=your-coturn-static-auth-secret
TURN_API_KEY=optional-api-key-for-turn-endpoint

# SFU
SFU_SECRET=your-sfu-jwt-secret

# Email (password reset)
EMAIL=your-email@gmail.com
EMAIL_PASSWORD=your-app-password
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
EMAIL_FROM=optional-from-address
FEEDBACK_EMAIL=optional-feedback-address
PUBLIC_BASE_URL=https://parchchat.com

# Optional email notification tuning
EMAIL_ACTIVITY_THRESHOLD=20
EMAIL_ACTIVITY_COOLDOWN_MINUTES=720
EMAIL_ACTIVITY_CHECK_MINUTES=10
EMAIL_WEEKLY_INTERVAL_DAYS=7
EMAIL_WEEKLY_CHECK_HOURS=24
```

### Email Notifications

Relay now supports:
- space invite emails
- high-activity message digest emails
- weekly product/feedback emails

Activity digest scope and privacy:
- digest counters are space-scoped (only spaces the user belongs to)
- relay does not store message content for digest emails (counts/timestamps only)

Every email includes unsubscribe links:
- per-category unsubscribe (`invite`, `activity`, `weekly`)
- global unsubscribe (`all`)

Unsubscribe endpoint:
- `GET /unsubscribe?token=<token>&type=invite|activity|weekly|all`

---

## Local Development

### Prerequisites
- Go 1.21+
- Node.js 18+
- npm or yarn

### Relay Server

```bash
cd relay_server
go mod tidy
go run .
```
Then open:
- `http://localhost:8000/` (landing page)
- `http://localhost:8000/client` (web chat client)
- `http://localhost:8000/call` (on-demand call landing)

### Call App (Video Calls)

```bash
cd call_app
npm install
npm run dev      # Development server at http://localhost:5173
npm run build    # Build to relay_server/static/call/
```

**Note:** For local development against production servers, edit `src/config/endpoints.ts`:
```typescript
const isDev = false; // Set to false to use production endpoints
```

### Desktop Client (Wails)

```bash
cd web_client/frontend
npm install
npm run dev:desktop-assets  # Build/watch desktop assets into web_client/frontend/dist
```
In another terminal:
```bash
cd web_client
wails dev
```

### Web Client (Browser)

```bash
cd web_client/frontend
npm install
npm run dev:web      # Vite dev server
npm run build:web    # Build to relay_server/static/client
```

---

## Database Migrations

### Host Client (SQLite)

```bash
# Create migration
migrate create -ext sql -dir ./migrations <name>

# Apply migrations
migrate -path ./migrations -database "sqlite3://chat.db?_foreign_keys=on" up

# Rollback
migrate -path ./migrations -database "sqlite3://chat.db?_foreign_keys=on" down
```

### Relay Server (SQLite)

Migrations are embedded and run automatically on startup.

---

## Contact

[parchchat@gmail.com](mailto:parchchat@gmail.com)
