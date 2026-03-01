package telegram

import (
	"fmt"
	"math"
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
// umbral is the reference price, diff = bid - umbral.
func FormatSpikeMessage(bid, umbral, diff float64, isUp bool) (string, tgbotapi.InlineKeyboardMarkup) {
	pct := (math.Abs(diff) / umbral) * 100
	now := time.Now().Format("02/01/2006 · 15:04:05")

	var title, dir string
	if isUp {
		title = "<blockquote>📈 USDT·BOB — Alerta de Subida</blockquote>\n\n🚨 <b>¡El USDT subió!</b>"
		dir = "▲ +"
	} else {
		title = "<blockquote>📉 USDT·BOB — Alerta de Bajada</blockquote>\n\n🚨 <b>¡El USDT bajó!</b>"
		dir = "▼ -"
	}

	lines := []string{
		row("Precio actual", fmt.Sprintf("%.4f BOB", bid)),
		row("Precio ref.", fmt.Sprintf("%.4f BOB", umbral)),
		divider(),
		row("Diferencia", fmt.Sprintf("%s%.4f BOB", dir, math.Abs(diff))),
		row("Variación", fmt.Sprintf("%s%.2f%%", dir, pct)),
		"",
		row("Exchange", "Binance P2P"),
		row("Par", "USDT / BOB"),
	}

	table := "<pre>" + strings.Join(lines, "\n") + "</pre>"

	text := strings.Join([]string{
		title,
		"",
		table,
		"",
		fmt.Sprintf("🕐 <i>%s</i>", now),
	}, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("ℹ️ Info detallada", siteURL),
		),
	)

	return text, btn
}

// FormatDailyMessage returns a clean daily-summary HTML message.
func FormatDailyMessage(bid float64) (string, tgbotapi.InlineKeyboardMarkup) {
	now := time.Now().Format("02/01/2006 · 15:04:05")

	lines := []string{
		row("Precio", fmt.Sprintf("%.4f BOB", bid)),
		row("Par", "USDT / BOB"),
		row("Exchange", "Binance P2P"),
	}

	table := "<pre>" + strings.Join(lines, "\n") + "</pre>"

	text := strings.Join([]string{
		"<blockquote>💱 USDT·BOB — Cotización del Día</blockquote>",
		"",
		fmt.Sprintf("<b>💵  1 USDT = %.4f BOB</b>", bid),
		"",
		table,
		"",
		fmt.Sprintf("🕐 <i>Actualizado: %s</i>", now),
	}, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("ℹ️ Info detallada", siteURL),
		),
	)

	return text, btn
}

// ── Bot actions ───────────────────────────────────────────────────────────────

// SendMessage sends a new HTML message and returns its Telegram message ID.
func (b *Bot) SendMessage(text string, silent bool, replyMarkup tgbotapi.InlineKeyboardMarkup) (int, error) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	msg.DisableNotification = silent
	msg.ReplyMarkup = replyMarkup

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("error sending message: %w", err)
	}

	return sent.MessageID, nil
}

// EditMessage replaces the content of an existing message.
func (b *Bot) EditMessage(messageID int, text string, replyMarkup tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageText(b.chatID, messageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.DisableWebPagePreview = true
	edit.ReplyMarkup = &replyMarkup

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
