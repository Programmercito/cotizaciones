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

## Core flow (main.go)

1. Fetch USDT/BOB from `https://criptoya.com/api/binancep2p/USDT/BOB`
2. Open SQLite DB (`modernc.org/sqlite`, WAL mode). Applies inline migrations: adds `purchase` and `umbral_referencial` columns if missing.
3. Insert quote with `time.Now()` as timestamp (avoids duplicate key issues from API caching).
4. Build summary from latest DB rows for: USDT, USD Oficial, USD Referencial, Euro, Oro, Plata, UFV.
5. Generate a temp PNG image (`telegram.GeneratePriceImage`) with QR codes. Sent to Telegram if possible; falls back to text. Temp file is deferred-removed.
6. Git: `ForcePull` frontend repo -> export JSON into it -> `CommitAndPush` with message `"data upload"`.
7. Delete DB records older than **60 days** (`2 * 30 * 24h`).

## Telegram logic (the easy-to-break part)

Two independent thresholds are tracked in the `config` table:
- `umbral` -> USDT reference
- `umbral_referencial` -> USD Referencial reference

Spike threshold in code: **`0.20`** (not 0.30).

Rules:
- If either threshold is `null` (first run or reset): save current prices as both thresholds, **skip Telegram entirely**, preserve existing `messageID`.
- No valid `messageID`: send a new daily summary message, save only `messageID`.
- `isOutside == true` (abs(diff) > 0.20 for **either** price): send a spike alert, update **both thresholds** to current prices, save new `messageID`.
- `isOutside == false`: edit the existing Telegram message. If edit fails, send new message and save only `messageID`. **Never update thresholds** in this path.

The spike message uses the larger of the two diffs for the visual alert text.

## DB schema notes

- Table `cotizaciones`: `moneda`, `cotizacion`, `purchase`, `datetime`, `exchange`, `moneda_dest`
- Table `config`: single row with `currentdate`, `chatid`, `messageid`, `umbral`, `umbral_referencial`
- `db.New()` applies WAL mode and best-effort column migrations on every open.

## CI / deploy

- `.github/workflows/go.yaml` builds the Linux binary and deploys via SSH to a server running microk8s.
- Deployment pulls Docker/k8s manifests from a separate repo (`bot-dockers`) on the server.
