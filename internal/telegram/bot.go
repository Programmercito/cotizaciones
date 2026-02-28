package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const siteURL = "https://cotizaciones.devcito.org/"

// Bot wraps the Telegram bot API bound to a specific chat.
type Bot struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

// New creates a new Bot instance validated against the Telegram API.
func New(token, chatID string) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
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

// FormatSpikeMessage returns a visually rich HTML alert for a price spike.
// weeklyAvg is the 7-day average (previous week), diff = bid − weeklyAvg.
func FormatSpikeMessage(bid, weeklyAvg, diff float64) string {
	pct := (diff / weeklyAvg) * 100
	now := time.Now().Format("02/01/2006 · 15:04:05")

	lines := []string{
		"� <b>¡El USDT subió significativamente!</b>",
		"",
		row("Precio actual", fmt.Sprintf("%.4f BOB", bid)),
		row("Prom. 7 días", fmt.Sprintf("%.4f BOB", weeklyAvg)),
		divider(),
		row("Diferencia", fmt.Sprintf("▲ +%.4f BOB", diff)),
		row("Variación", fmt.Sprintf("▲ +%.2f%%", pct)),
		"",
		row("Exchange", "Binance P2P"),
		row("Par", "USDT / BOB"),
	}

	table := "<pre>" + strings.Join(lines, "\n") + "</pre>"

	return strings.Join([]string{
		"<blockquote>📈 USDT·BOB — Alerta de Subida</blockquote>",
		"",
		table,
		"",
		fmt.Sprintf("🕐 <i>%s</i>", now),
		fmt.Sprintf(`📊 <a href="%s">Ver historial completo</a>`, siteURL),
	}, "\n")
}

// FormatDailyMessage returns a clean daily-summary HTML message.
func FormatDailyMessage(bid float64) string {
	now := time.Now().Format("02/01/2006 · 15:04:05")

	lines := []string{
		row("Precio", fmt.Sprintf("%.4f BOB", bid)),
		row("Par", "USDT / BOB"),
		row("Exchange", "Binance P2P"),
	}

	table := "<pre>" + strings.Join(lines, "\n") + "</pre>"

	return strings.Join([]string{
		"<blockquote>💱 USDT·BOB — Cotización del Día</blockquote>",
		"",
		fmt.Sprintf("<b>💵  1 USDT = %.4f BOB</b>", bid),
		"",
		table,
		"",
		fmt.Sprintf("🕐 <i>Actualizado: %s</i>", now),
		fmt.Sprintf(`📊 <a href="%s">Ver historial completo</a>`, siteURL),
	}, "\n")
}

// ── Bot actions ───────────────────────────────────────────────────────────────

// SendMessage sends a new HTML message and returns its Telegram message ID.
func (b *Bot) SendMessage(text string) (int, error) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("error sending message: %w", err)
	}

	return sent.MessageID, nil
}

// EditMessage replaces the content of an existing message.
func (b *Bot) EditMessage(messageID int, text string) error {
	edit := tgbotapi.NewEditMessageText(b.chatID, messageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.DisableWebPagePreview = true

	if _, err := b.api.Send(edit); err != nil {
		return fmt.Errorf("error editing message: %w", err)
	}

	return nil
}

// ── Layout helpers ────────────────────────────────────────────────────────────

const colWidth = 14 // label column width (chars)

// row formats a label/value pair into a fixed-width table row.
func row(label, value string) string {
	pad := colWidth - len([]rune(label))
	if pad < 1 {
		pad = 1
	}
	return fmt.Sprintf("%s%s%s", label, strings.Repeat(" ", pad), value)
}

// divider returns a thin separator line matching the table width.
func divider() string {
	return strings.Repeat("─", 28)
}
