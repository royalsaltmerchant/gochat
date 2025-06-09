# Parch

**The user-hosted messenger and voice chat application.**

Parch is a real-time chat and voice platform built around decentralization and simplicity. It allows users to run their own servers (hosts), connect through a public relay, and communicate securely via self-hosted infrastructure.

---

## Architecture

Parch consists of multiple cooperating components:

- **Relay Server**  
  A central Go service responsible for handling host and user registrations. It acts as a lightweight signal relay between clients and hosts, facilitating connection setup.

- **Host Application**  
  A desktop application that stores chat data locally using SQLite. It manages *spaces*, *channels*, and message storage. Hosts register with the relay and provide a **Host Key** to clients for access.  
  (Currently uses `andlabs/ui` for Windows and macOS interfaces.)

- **Client Application**  
  The main user-facing chat app. Users connect to hosts using Host Keys and participate in channel-based communication within user-created spaces.

- **SFU (Selective Forwarding Unit)**  
  A Pion-based media server that routes voice streams between participants without mixing them, improving performance for group audio.

- **TURN Server**  
  Enables NAT traversal for clients behind restrictive firewalls, ensuring reliable peer-to-peer and media connections over WebRTC.

---

## Getting Started

### 📦 Download Binaries

Beta builds are available for macOS and Windows on the [landing page](https://github.com/royalsaltmerchant/gochat):

- **Parch Client** (macOS & Windows)
- **Parch Host** (macOS & Windows)

---

##  Local Development

Ensure you have Go installed and available in your environment.

### Install Dependencies

```bash
go mod tidy
```

### Start the Relay Server

```bash
go run ./relay_server
```

### Run the Host Application

```bash
go run ./host_client
```

### Run the Web Client (Wails)

```bash
cd web_client
wails dev
```

To build the frontend:

```bash
cd frontend
npm run build
```

---

## ⚙️ Database Migrations (Host)

Parch Host uses SQLite for data storage and `golang-migrate` for schema evolution.

### Create a New Migration

```bash
migrate create -ext sql -dir ./migrations <name>
```

### Apply Migrations

```bash
migrate -path ./migrations -database "sqlite3://chat.db?_foreign_keys=on" up
```

### Rollback Migrations

```bash
migrate -path ./migrations -database "sqlite3://chat.db?_foreign_keys=on" down
```

### Troubleshooting

If you encounter SQLite build errors:

```bash
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## Contact

[parchchat@gmail.com](mailto:parchchat@gmail.com)

