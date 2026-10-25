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
  - `/opt/osbo/cotiza/usd.jpg` (USDT + USD Oficial + USD Referencial)
  - `/opt/osbo/cotiza/otros.jpg` (Euro + Oro + Plata + UFV)

## Core flow (main.go)

1. Fetch USDT/BOB from `https://criptoya.com/api/binancep2p/USDT/BOB`
2. Open SQLite DB (`modernc.org/sqlite`, WAL mode). Applies inline migrations: adds `purchase` and `umbral_referencial` and `messageidusd` columns if missing.
3. Insert quote with `time.Now()` as timestamp (avoids duplicate key issues from API caching).
4. Build summary from latest DB rows for: USDT, USD Oficial, USD Referencial, Euro, Oro, Plata, UFV.
5. Generate **two** fixed JPG images in `/opt/osbo/cotiza/` (`usd.jpg` and `otros.jpg`) with QR codes. Sent to Telegram if possible; falls back to text.
6. Git: `ForcePull` frontend repo -> export JSON into it -> `CommitAndPush` with message `"data upload"`.
7. Delete DB records older than **90 days** (`3 * 30 * 24h`).

## Telegram logic (the easy-to-break part)

Robustness rules (since 2026-10): the bot HTTP client has explicit timeouts
(30s request, 10s dial) so a hung connection can't freeze a cron slot; edits
retry transient failures (network/429/5xx) up to 3 attempts, NEW SENDS retry
only on 429 (a network timeout may already have delivered the message —
retrying would duplicate it); `message is not modified` counts as SUCCESS
(never triggers a new message); edits fall back through a chain
(EditPhoto -> EditMessage -> EditCaption) because the stored messageID may
point to a photo or a text message; only if ALL edits fail is a new message
sent. "chat not found"/"bot is not a member" exits non-zero so the CronJob
records the failure. DB-sourced strings (MonedaDest, raw datetimes) are
HTML-escaped; photo captions are clamped to 1024 visible chars and texts to
4096 before sending. Stored messageIDs must parse as positive ints or are
treated as absent (self-heals). Images are written atomically (temp + rename)
and overlapping runs are blocked by a flock at `/opt/osbo/cotizaciones.lock`
(skip = exit quietly). Spike detection ignores USD Referencial when
`showReferencial` is false or the row is missing (retired 2026-06-30; a
stale/0 value used to fake spikes). If `GetLatestSummary` fails, Telegram is
skipped entirely. Config queries use `ORDER BY rowid` for read/write
determinism. Since 2026-10-24 the text messages use compact quote blocks
(`quoteRow`): head line with market + HH:MM, one price line, and a
`▲/▼/● vs ayer` delta computed from `USDBaselines` (shared `buildTrend` with
the images). `Compra` is hidden when the quote tracks no purchase (only USDT
does); the trend line is hidden when no baseline exists — no fake zeros.

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
