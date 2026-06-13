# Agents

Compact reference for the `cotizaciones` Go app.

## Build & run

- Single binary: `main.go` at repo root. Build: `go build -o cotizaciones .`
- No tests in this repo. There is a small DB debug util at `cmd/dbcheck/main.go`.
- Requires Go 1.25+ (`go.mod`). CI builds with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`.

## Environment

- `TELEGRAM_BOT_TOKEN` is required. Loaded from `.env` via `godotenv` if present; `.env` is gitignored.
- No other runtime flags. Hardcoded paths below are production defaults.

## Hardcoded production paths

- SQLite DB: `/opt/osbo/datausd`
- Frontend repo: `/opt/codes/cotizaciones_ng`
- JSON export: `/opt/codes/cotizaciones_ng/docs/data.json`
- Telegram images (fixed, overwritten each run):
  - `/opt/osbo/cotiza/usdt.png` (USDT + USD Oficial + USD Referencial)
  - `/opt/osbo/cotiza/resto.png` (Euro + Oro + Plata + UFV)

## Core flow (main.go)

1. Fetch USDT/BOB from `https://criptoya.com/api/binancep2p/USDT/BOB`
2. Open SQLite DB (`modernc.org/sqlite`, WAL mode). Applies inline migrations: adds `purchase` and `umbral_referencial` and `messageidusd` columns if missing.
3. Insert quote with `time.Now()` as timestamp (avoids duplicate key issues from API caching).
4. Build summary from latest DB rows for: USDT, USD Oficial, USD Referencial, Euro, Oro, Plata, UFV.
5. Generate **two** fixed PNG images in `/opt/osbo/cotiza/` (`usdt.png` and `resto.png`) with QR codes. Sent to Telegram if possible; falls back to text.
6. Git: `ForcePull` frontend repo -> export JSON into it -> `CommitAndPush` with message `"data upload"`.
7. Delete DB records older than **60 days** (`2 * 30 * 24h`).

## Telegram logic (the easy-to-break part)

Two independent messages are sent/edited:

**USD message** (`messageidusd`): USDT + USD Oficial + USD Referencial
- Tracks thresholds `umbral` (USDT) and `umbral_referencial` (USD Referencial).
- Spike threshold: **0.20**.
- If either threshold is `null`: save current prices as both thresholds, **skip USD message only**, preserve `messageidusd`.
- No valid `messageidusd`: send new USD summary, save only `messageidusd`.
- `isOutside == true` for USDT or USD Referencial: send spike alert to USD message, **also send a new Resto message** so the pair stays together in the chat, update both thresholds, save new `messageid` and `messageidusd`.
- `isOutside == false`: edit existing USD message. If edit fails, send new USD message and save only `messageidusd`. Never update thresholds.

**Resto message** (`messageid`): Euro + Oro + Plata + UFV
- No thresholds, no spike logic.
- No valid `messageid`: send new Resto summary, save only `messageid`.
- Valid `messageid`: edit existing Resto message. If edit fails, send new and save only `messageid`.
- During a USD spike the Resto message is always re-sent together with the USD spike alert so both messages remain paired in the chat.

## DB schema notes

- Table `cotizaciones`: `moneda`, `cotizacion`, `purchase`, `datetime`, `exchange`, `moneda_dest`
- Table `config`: single row with `currentdate`, `chatid`, `messageid`, `messageidusd`, `umbral`, `umbral_referencial`
- `db.New()` applies WAL mode and best-effort column migrations on every open.

## CI / deploy

- `.github/workflows/go.yaml` builds the Linux binary and deploys via SSH to a server running microk8s.
- Deployment pulls Docker/k8s manifests from a separate repo (`bot-dockers`) on the server.
