# Parch

Real-time self-hosted messenger and voice chat application.

## Dev
Install deps
`go mod tidy`

### Relay

`go run ./relay_server`

### Host Client

`go run ./host_client`

### Web Client

`cd web_client; wails dev`

`cd frontend; npm run build`

## Env

## DB Migrations

### New Migration

`migrate create -ext sql -dir <migrations dir> <table name>
`

### Example up and down

`migrate -path migrations -database "sqlite3://mydb.sqlite?_foreign_keys=on" up
`

`migrate -path migrations -database "sqlite3://mydb.sqlite?_foreign_keys=on" down
`

## Notes

had issues installing go migrate with sqlite, trying this:

`go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
