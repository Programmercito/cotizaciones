package telegram

import (
	"cotizaciones/internal/db"
	"cotizaciones/internal/ui"
	"errors"
	"fmt"
	"html"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const siteURL = "https://cotizaciones.devcito.org/"

// fmtDT convierte un datetime de la DB al formato legible para Telegram.
// El valor de salida se usa dentro de HTML: si no se puede parsear se
// escapa el crudo para que un dato exotico no rompa el ParseMode.
func fmtDT(dt string) string {
	layouts := []string{
		db.TimeFmt,
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dt)
		if err != nil {
			continue
		}
		if layout == db.TimeFmt || layout == "2006-01-02 15:04" {
			return t.Format(db.DisplayTimeFmt)
		}
		return t.Format(db.DisplayDateFmt)
	}

	return html.EscapeString(dt)
}

// fmtDest returns a formatted moneda destino tag, or empty if blank.
// The destination comes from the DB and may contain HTML-special characters;
// escaping it prevents "can't parse entities" 400 errors on edits/sends.
func fmtDest(dest string) string {
	if dest == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", html.EscapeString(dest))
}

// ShowReferencial returns true while the USD Referencial quote should still be
// rendered in Telegram output. It becomes false at 2026-06-30 00:00:00 server local time.
func ShowReferencial(now time.Time) bool {
	cutoff := time.Date(2026, 6, 30, 0, 0, 0, 0, time.Local)
	return now.Before(cutoff)
}

// Bot wraps the Telegram bot API bound to a specific chat.
type Bot struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

// New creates a new Bot instance validated against the Telegram API.
// The HTTP client carries explicit timeouts: the library default (&http.Client{})
// never times out, so one hung TCP connection would freeze the run forever —
// and the flock would then make every later cron run skip silently.
func New(token, chatID string) (*Bot, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram bot: %w", err)
	}

	cid, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	return &Bot{api: bot, chatID: cid}, nil
}

// ── Message formatters ────────────────────────────────────────────────────────

// FormatUSDMessage returns a clean daily-summary HTML message for USDT/USD.
func FormatUSDMessage(summary map[string]db.Cotizacion, showReferencial bool) (string, tgbotapi.InlineKeyboardMarkup) {
	usdt := summary["USDT"]
	oficial := summary["usd oficial"]
	generatedAt := time.Now().Format(db.DisplayTimeFmt)

	lines := []string{
		"<blockquote><b>☀️ Resumen USD / USDT</b></blockquote>",
		"🏛️ <b>Mercados:</b> Binance P2P / BCB",
		"",
		"💰 <b>USDT (Binance):</b>" + fmtDest(usdt.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", usdt.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(usdt.Datetime)),
		"",
		"🏢 <b>BCB - USD Oficial:</b>" + fmtDest(oficial.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", oficial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", oficial.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(oficial.Datetime)),
	}

	if showReferencial {
		referencial := summary["usd referencial"]
		lines = append(lines, "",
			"📊 <b>BCB - USD Referencial:</b>"+fmtDest(referencial.MonedaDest),
			fmt.Sprintf("💵 Venta:  <code>%.2f</code>", referencial.Cotizacion),
			fmt.Sprintf("🛒 Compra: <code>%.2f</code>", referencial.Purchase),
			fmt.Sprintf("🕒 <i>%s</i>", fmtDT(referencial.Datetime)),
		)
	}

	lines = append(lines, "", fmt.Sprintf("📅 <i>Generado: %s</i>", generatedAt))
	text := strings.Join(lines, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("💸 Ver detalles en la Web", siteURL),
		),
	)

	return text, btn
}

// FormatSpikeUSDMessage returns a visually rich HTML alert for a USD/USDT price spike.
func FormatSpikeUSDMessage(summary map[string]db.Cotizacion, umbralUSDT, diffUSDT, umbralRef, diffRef float64, showReferencial bool) (string, tgbotapi.InlineKeyboardMarkup) {
	usdt := summary["USDT"]
	official := summary["usd oficial"]
	officialMonedaDest := fmtDest(official.MonedaDest)
	generatedAt := time.Now().Format(db.DisplayTimeFmt)

	// Determine the dominant direction from the largest absolute change.
	mainDiff := diffUSDT
	if math.Abs(diffRef) > math.Abs(diffUSDT) {
		mainDiff = diffRef
	}
	isUp := mainDiff > 0

	var title, emoji, trend string
	if isUp {
		title = "<blockquote><b>🚀 ¡SUBIDA DE PRECIO! | USD/USDT</b></blockquote>"
		emoji = "📈"
		trend = "Subida rápida"
	} else {
		title = "<blockquote><b>🔻 ¡BAJADA DE PRECIO! | USD/USDT</b></blockquote>"
		emoji = "📉"
		trend = "Caída rápida"
	}

	pct := func(diff, base float64) float64 {
		if base == 0 {
			return 0
		}
		return (math.Abs(diff) / base) * 100
	}

	sign := func(v float64) string {
		if v < 0 {
			return "-"
		}
		return "+"
	}

	lines := []string{
		title,
		fmt.Sprintf("%s <b>Tendencia:</b> %s", emoji, trend),
		"🏛️ <b>Mercado:</b> Binance P2P / BCB",
		"",
		"💰 <b>USDT (Binance):</b>" + fmtDest(usdt.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", usdt.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(usdt.Datetime)),
		"",
		"🏢 <b>BCB - USD Oficial:</b>" + officialMonedaDest,
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", official.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", official.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(official.Datetime)),
	}

	if showReferencial {
		referencial := summary["usd referencial"]
		lines = append(lines, "",
			"📊 <b>BCB - USD Referencial:</b>"+fmtDest(referencial.MonedaDest),
			fmt.Sprintf("💵 Venta:  <code>%.2f</code>", referencial.Cotizacion),
			fmt.Sprintf("🛒 Compra: <code>%.2f</code>", referencial.Purchase),
			fmt.Sprintf("🕒 <i>%s</i>", fmtDT(referencial.Datetime)),
		)
	}

	variationLines := []string{
		"",
		"────────────────────────",
		fmt.Sprintf("📊 Variación USDT: <code>%s%.2f</code> (<code>%s%.2f%%</code>)", sign(diffUSDT), math.Abs(diffUSDT), sign(diffUSDT), pct(diffUSDT, umbralUSDT)),
		fmt.Sprintf("🏷️ Ref. Anterior USDT: <code>%.2f</code>", umbralUSDT),
	}

	if showReferencial {
		variationLines = append(variationLines,
			fmt.Sprintf("📊 Variación USD Referencial: <code>%s%.2f</code> (<code>%s%.2f%%</code>)", sign(diffRef), math.Abs(diffRef), sign(diffRef), pct(diffRef, umbralRef)),
			fmt.Sprintf("🏷️ Ref. Anterior USD Ref: <code>%.2f</code>", umbralRef),
		)
	}

	variationLines = append(variationLines, fmt.Sprintf("📅 <i>Generado: %s</i>", generatedAt))
	lines = append(lines, variationLines...)
	text := strings.Join(lines, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("💸 Ver detalles en la Web", siteURL),
		),
	)

	return text, btn
}

// FormatRestoMessage returns a clean daily-summary HTML message for Euro/Oro/Plata/UFV.
func FormatRestoMessage(summary map[string]db.Cotizacion) (string, tgbotapi.InlineKeyboardMarkup) {
	euro := summary["eur"]
	oro := summary["oro"]
	plata := summary["plata"]
	ufv := summary["ufv"]
	generatedAt := time.Now().Format(db.DisplayTimeFmt)

	text := strings.Join([]string{
		"<blockquote><b>☀️ Resumen Cotizaciones</b></blockquote>",
		"🏛️ <b>Mercados:</b> BCB / Internacional",
		"",
		"🇪🇺 <b>Euro:</b>" + fmtDest(euro.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", euro.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", euro.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(euro.Datetime)),
		"",
		"🥇 <b>Oro (Troy Oz):</b>" + fmtDest(oro.MonedaDest),
		fmt.Sprintf("💵 Precio: <code>%.2f</code>", oro.Cotizacion),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(oro.Datetime)),
		"",
		"🥈 <b>Plata (Troy Oz):</b>" + fmtDest(plata.MonedaDest),
		fmt.Sprintf("💵 Precio: <code>%.2f</code>", plata.Cotizacion),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(plata.Datetime)),
		"",
		"📐 <b>UFV:</b>" + fmtDest(ufv.MonedaDest),
		fmt.Sprintf("💵 Valor:  <code>%.2f</code>", ufv.Cotizacion),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(ufv.Datetime)),
		"",
		fmt.Sprintf("📅 <i>Generado: %s</i>", generatedAt),
	}, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("💸 Ver detalles en la Web", siteURL),
		),
	)

	return text, btn
}

// ── Bot actions ───────────────────────────────────────────────────────────────

// isNotModified reports whether Telegram rejected an edit because the new
// content is identical to the current one. This is a success signal for us:
// the message already shows what we want, so we must NOT send a new message.
func isNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message is not modified")
}

// isTransient reports whether the error is worth retrying: Telegram 429 rate
// limits, 5xx server errors, or plain network failures (DNS, timeouts, etc.).
func isTransient(err error) bool {
	if err == nil || isNotModified(err) {
		return false
	}
	var tgErr tgbotapi.Error
	if errors.As(err, &tgErr) {
		return tgErr.Code == 429 || tgErr.Code >= 500
	}
	// Non-API errors are network level: transient by definition.
	return true
}

// retryAfter extracts Telegram's suggested retry delay from a 429 response.
func retryAfter(err error) time.Duration {
	var tgErr tgbotapi.Error
	if errors.As(err, &tgErr) && tgErr.RetryAfter > 0 {
		return time.Duration(tgErr.RetryAfter) * time.Second
	}
	return 0
}

// ErrChatNotFound marks a permanent failure where the bot can no longer write
// to the configured chat (chat deleted, bot kicked from the group, wrong
// chatid). Retrying or sending new messages is pointless; main treats it as
// fatal so the CronJob records a failed run and the operator notices.
var ErrChatNotFound = errors.New("chat not found")

// IsChatGone reports whether err is an ErrChatNotFound wrapped by callers.
func IsChatGone(err error) bool {
	return errors.Is(err, ErrChatNotFound)
}

func isChatGone(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "chat not found") || strings.Contains(msg, "bot is not a member")
}

// visibleLen counts the characters Telegram measures: the text with HTML
// tags stripped (Telegram parses entities before length-checking).
func visibleLen(text string) int {
	var b strings.Builder
	b.Grow(len(text))
	inTag := false
	for _, r := range text {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return utf8.RuneCountInString(b.String())
}

// clampCaption trims whole trailing lines until the message fits maxVisible
// characters as Telegram counts them. Lines are self-contained here (no tag
// spans a newline), so trimming at line boundaries keeps the HTML valid.
// If even the first line is too long it degrades to escaped plain text.
func clampCaption(text string, maxVisible int) string {
	if visibleLen(text) <= maxVisible {
		return text
	}
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i > 0; i-- {
		candidate := strings.Join(lines[:i], "\n")
		if visibleLen(candidate) <= maxVisible {
			ui.Warn(fmt.Sprintf("Caption recortada de %d a %d caracteres visibles para respetar el limite de Telegram", visibleLen(text), visibleLen(candidate)))
			return candidate
		}
	}
	ui.Warn(fmt.Sprintf("Caption demasiado larga (primer linea > %d): se envia como texto plano", maxVisible))
	return html.EscapeString(firstRunes(strings.Join(lines, " "), maxVisible))
}

func firstRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// retryable decides whether an API error is worth another attempt.
// Idempotent calls (edits) retry any transient failure: network, 429, 5xx.
// Non-idempotent calls (new sends) retry ONLY 429, which guarantees Telegram
// did not process the request. A network timeout or 5xx on a send may mean
// the message was already delivered; retrying would post a duplicate and
// orphan its message ID.
func retryable(err error, idempotent bool) bool {
	if !isTransient(err) {
		return false
	}
	if idempotent {
		return true
	}
	var tgErr tgbotapi.Error
	return errors.As(err, &tgErr) && tgErr.Code == 429
}

// send calls the Telegram API retrying when retryable() says it is safe,
// with backoff that honors Telegram's retry_after. "message is not modified"
// and unrecoverable errors are surfaced untouched to the caller.
func (b *Bot) send(cfg tgbotapi.Chattable, idempotent bool) (tgbotapi.Message, error) {
	const maxAttempts = 3
	delay := 2 * time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		msg, err := b.api.Send(cfg)
		if err == nil {
			return msg, nil
		}
		if isChatGone(err) {
			return msg, fmt.Errorf("%w: %v", ErrChatNotFound, err)
		}
		if !retryable(err, idempotent) {
			return msg, err
		}
		lastErr = err
		if attempt < maxAttempts {
			wait := delay
			if ra := retryAfter(err); ra > wait {
				wait = ra
			}
			ui.Warn(fmt.Sprintf("Falla transitoria de Telegram (intento %d/%d, reintento en %s): %v", attempt, maxAttempts, wait, err))
			time.Sleep(wait)
			delay *= 2
		}
	}
	return tgbotapi.Message{}, lastErr
}

// SendMessage sends a new HTML message and returns its Telegram message ID.
func (b *Bot) SendMessage(text string, silent bool, replyMarkup tgbotapi.InlineKeyboardMarkup) (int, error) {
	msg := tgbotapi.NewMessage(b.chatID, clampCaption(text, 4096))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	msg.DisableNotification = silent
	msg.ReplyMarkup = replyMarkup

	sent, err := b.send(msg, false)
	if err != nil {
		return 0, fmt.Errorf("error sending message: %w", err)
	}

	return sent.MessageID, nil
}

// EditMessage replaces the content of an existing text message.
// A "message is not modified" reply from Telegram is treated as success.
func (b *Bot) EditMessage(messageID int, text string, replyMarkup tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageText(b.chatID, messageID, clampCaption(text, 4096))
	edit.ParseMode = tgbotapi.ModeHTML
	edit.DisableWebPagePreview = true
	edit.ReplyMarkup = &replyMarkup

	if _, err := b.send(edit, true); err != nil {
		if isNotModified(err) {
			return nil
		}
		return fmt.Errorf("error editing message: %w", err)
	}

	return nil
}

// SendPhoto sends a photo message with caption and returns its Telegram message ID.
func (b *Bot) SendPhoto(imagePath, caption string, silent bool, replyMarkup tgbotapi.InlineKeyboardMarkup) (int, error) {
	photo := tgbotapi.NewPhoto(b.chatID, tgbotapi.FilePath(imagePath))
	photo.Caption = clampCaption(caption, 1024)
	photo.ParseMode = tgbotapi.ModeHTML
	photo.DisableNotification = silent
	photo.ReplyMarkup = replyMarkup

	sent, err := b.send(photo, false)
	if err != nil {
		return 0, fmt.Errorf("error sending photo: %w", err)
	}

	return sent.MessageID, nil
}

// EditPhoto replaces an existing photo message with a new image and caption.
// A "message is not modified" reply from Telegram is treated as success.
func (b *Bot) EditPhoto(messageID int, imagePath, caption string, replyMarkup tgbotapi.InlineKeyboardMarkup) error {
	media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(imagePath))
	media.Caption = clampCaption(caption, 1024)
	media.ParseMode = tgbotapi.ModeHTML

	edit := tgbotapi.EditMessageMediaConfig{
		BaseEdit: tgbotapi.BaseEdit{ChatID: b.chatID, MessageID: messageID, ReplyMarkup: &replyMarkup},
		Media:    media,
	}

	if _, err := b.send(edit, true); err != nil {
		if isNotModified(err) {
			return nil
		}
		return fmt.Errorf("error editing photo message: %w", err)
	}

	return nil
}

// EditCaption updates only the caption of an existing photo message without
// re-uploading the image. It is the fallback for editing a photo message when
// no image is available in this run, and it is treated as success when
// Telegram replies "message is not modified".
func (b *Bot) EditCaption(messageID int, caption string, replyMarkup tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageCaption(b.chatID, messageID, clampCaption(caption, 1024))
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &replyMarkup

	if _, err := b.send(edit, true); err != nil {
		if isNotModified(err) {
			return nil
		}
		return fmt.Errorf("error editing photo caption: %w", err)
	}

	return nil
}
