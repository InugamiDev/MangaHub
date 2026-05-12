# CLI Reference

## Purpose

The CLI lives at `services/api/cmd/mangahub`. It implements the production-critical subset of the original manual: auth, catalog lookup, library/progress updates, TCP progress monitoring, UDP notification subscription, WebSocket chat send, gRPC manga/progress calls, and server health.

## System Requirements

- Go 1.26, matching the current `services/api/go.mod`. The reference PDF lists Go 1.19 as the original baseline, but this repo now builds against the newer module version.
- SQLite 3.x.
- Network connectivity for sync features.
- UTF-8 terminal.
- Linux, macOS, and Windows support.

## Default Config

The CLI stores local configuration at `~/.mangahub/config.yaml`, matching the reference manual.

```yaml
server:
  host: "localhost"
  http_port: 8080
  tcp_port: 9090
  udp_port: 9091
  grpc_port: 9092
  websocket_port: 9093

database:
  path: "~/.mangahub/data.db"

user:
  username: ""
  token: ""

sync:
  auto_sync: true
  conflict_resolution: "last_write_wins"

notifications:
  enabled: true
  sound: false

logging:
  level: "info"
  path: "~/.mangahub/logs/"
```

Environment overrides:

- `MANGAHUB_API_URL`
- `MANGAHUB_TCP_ADDR`
- `MANGAHUB_UDP_ADDR`
- `MANGAHUB_GRPC_ADDR`
- `MANGAHUB_WS_URL`
- `MANGAHUB_PASSWORD`
- `MANGAHUB_ADMIN_TOKEN` or `ADMIN_SYNC_TOKEN` for admin mutation commands

## Command Pattern

```bash
mangahub <command> <subcommand> [flags] [arguments]
```

Build or run from `services/api`:

```bash
go run ./cmd/mangahub version
go build -o mangahub ./cmd/mangahub
```

## Core Commands

- `mangahub init`
- `mangahub version`
- `mangahub auth register --username <username> --email <email>`
- `mangahub auth login --username <username>`
- `mangahub auth login --email <email>`
- `mangahub auth logout`
- `mangahub auth status`
- `mangahub manga search <query>`
- `mangahub manga info <manga-id>`
- `mangahub manga list`
- `mangahub library add --manga-id <id> --status <status>`
- `mangahub library list`
- `mangahub library remove --manga-id <id>`
- `mangahub progress update --manga-id <id> --chapter <number>`

## Network Commands

- `mangahub sync monitor`
- `mangahub notify subscribe`
- `mangahub grpc manga get --id <manga-id>`
- `mangahub grpc manga search --query <search-term>`
- `mangahub grpc progress update --manga-id <id> --chapter <number>`
- `mangahub chat send "Hello everyone!"`
- `mangahub chat send "Great chapter!" --manga-id one-piece`

## Server Commands

- `mangahub server health`
- `mangahub server status`
- `mangahub server start` recognized stub, not implemented in local CLI
- `mangahub server stop` recognized stub, not implemented in local CLI
- `mangahub server logs` recognized stub, not implemented in local CLI

## Admin Commands

Admin commands call the protected `/admin` API routes. Set `MANGAHUB_ADMIN_TOKEN` or `ADMIN_SYNC_TOKEN` before running them.

- `mangahub admin manga create --title <title> --author <author> --genres "Action,Shounen"`
- `mangahub admin manga update --id <manga-id> --status completed --total-chapters 12`
- `mangahub admin manga delete --id <manga-id>`
- `mangahub admin chapter create --manga-id <manga-id> --id chapter-1 --title Opening --number 1 --pages "https://cdn.example.com/page1.jpg" --publish-status published`
- `mangahub admin chapter update --manga-id <manga-id> --id chapter-1 --title "Opening Revised"`
- `mangahub admin chapter delete --manga-id <manga-id> --id chapter-1`
- `mangahub admin notify send --manga-id <manga-id> --message "New chapter released"`
- `mangahub admin sync anilist --query "one piece" --limit 5`

## Test Console

The CLI test console prints pass/fail/skip metrics with latency in milliseconds. It checks HTTP health, HTTP manga search, gRPC search, UDP ping, TCP authenticated ping, and WebSocket authenticated ping. TCP and WebSocket checks are skipped when no user token is configured.

```bash
mangahub test console
mangahub test console --json
mangahub test console --skip-protocols
npm run api:console
```

## Advanced Commands

The original PDF lists config, profile, stats, backup, export, log, database repair, and update commands as extended operations. The local CLI recognizes these manual-coverage commands and returns a clear `not implemented in the local CLI` error when no backend exists.

- `mangahub config show`
- `mangahub config set <api_url|tcp_addr|udp_addr|grpc_addr|ws_url|username|token> <value>`
- `mangahub config reset`
- `mangahub profile list`
- `mangahub profile create <name>` recognized stub
- `mangahub profile switch <name>` recognized stub
- `mangahub stats overview` recognized stub
- `mangahub export library`
- `mangahub export progress` recognized stub
- `mangahub export all` recognized stub
- `mangahub backup create` recognized stub
- `mangahub backup restore` recognized stub
- `mangahub db check` recognized stub
- `mangahub db optimize` recognized stub
- `mangahub db stats` recognized stub
- `mangahub db repair` recognized stub
- `mangahub logs errors` recognized stub
- `mangahub logs search` recognized stub
- `mangahub logs clean` recognized stub
- `mangahub update check` recognized stub
- `mangahub update install` recognized stub

## Troubleshooting Expectations

- Server connection failures should suggest checking `server status`, starting services, firewall settings, config, and logs.
- TCP port conflicts should report port `9090` already in use.
- UDP with zero clients should be a warning, not a fatal error.
- Database repair should detect orphaned progress entries and rebuild indexes when possible.
- Critical database corruption should suggest backup restore, reinitialization, or exporting recoverable data.
