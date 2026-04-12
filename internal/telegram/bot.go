package telegram

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"cotizaciones/internal/db"

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
func FormatSpikeMessage(summary map[string]db.Cotizacion, umbral, diff float64, isUp bool) (string, tgbotapi.InlineKeyboardMarkup) {
	usdt := summary["USDT"]
	pct := (math.Abs(diff) / umbral) * 100
	now := time.Now().Format("02/01/2006 · 15:04:05")

	var title, dir, emoji, trend string
	if isUp {
		title = "<blockquote><b>🚀 ¡SUBIDA DE PRECIO! | USDT</b></blockquote>"
		emoji = "📈"
		dir = "+"
		trend = "Subida rápida"
	} else {
		title = "<blockquote><b>🔻 ¡BAJADA DE PRECIO! | USDT</b></blockquote>"
		emoji = "📉"
		dir = "-"
		trend = "Caída rápida"
	}

	text := strings.Join([]string{
		title,
		fmt.Sprintf("%s <b>Tendencia:</b> %s", emoji, trend),
		"🏛️ <b>Mercado:</b> Binance P2P",
		"",
		"💰 <b>USDT (Binance):</b>",
		fmt.Sprintf("💵 Venta:  <code>%.4f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.4f</code>", usdt.Purchase),
		"",
		"🏢 <b>BCB - USD Oficial:</b>",
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", summary["usd oficial"].Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", summary["usd oficial"].Purchase),
		"",
		"📊 <b>BCB - USD Referencial:</b>",
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", summary["usd referencial"].Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", summary["usd referencial"].Purchase),
		"────────────────────────",
		fmt.Sprintf("📊 Variación USDT: <code>%s%.4f</code> (<code>%s%.2f%%</code>)", dir, math.Abs(diff), dir, pct),
		fmt.Sprintf("🏷️ Ref. Anterior: <code>%.4f</code>", umbral),
		"",
		fmt.Sprintf("🕒 <i>%s</i>", now),
	}, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("💸 Ver detalles en la Web", siteURL),
		),
	)

	return text, btn
}

// FormatDailyMessage returns a clean daily-summary HTML message.
func FormatDailyMessage(summary map[string]db.Cotizacion) (string, tgbotapi.InlineKeyboardMarkup) {
	now := time.Now().Format("02/01/2006 · 15:04:05")
	usdt := summary["USDT"]
	oficial := summary["usd oficial"]
	referencial := summary["usd referencial"]

	text := strings.Join([]string{
		"<blockquote><b>☀️ Resumen de Cotizaciones</b></blockquote>",
		"🏛️ <b>Mercados:</b> Binance P2P / BCB",
		"",
		"💰 <b>USDT (Binance):</b>",
		fmt.Sprintf("💵 Venta:  <code>%.4f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.4f</code>", usdt.Purchase),
		"",
		"🏢 <b>BCB - USD Oficial:</b>",
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", oficial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", oficial.Purchase),
		"",
		"📊 <b>BCB - USD Referencial:</b>",
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", referencial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", referencial.Purchase),
		"",
		fmt.Sprintf("🕒 <i>Actualizado: %s</i>", now),
	}, "\n")

	btn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("💸 Ver detalles en la Web", siteURL),
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

// SendPhoto sends a photo message with caption and returns its Telegram message ID.
func (b *Bot) SendPhoto(imagePath, caption string, silent bool, replyMarkup tgbotapi.InlineKeyboardMarkup) (int, error) {
	photo := tgbotapi.NewPhoto(b.chatID, tgbotapi.FilePath(imagePath))
	photo.Caption = caption
	photo.ParseMode = tgbotapi.ModeHTML
	photo.DisableNotification = silent
	photo.ReplyMarkup = replyMarkup

	sent, err := b.api.Send(photo)
	if err != nil {
		return 0, fmt.Errorf("error sending photo: %w", err)
	}

	return sent.MessageID, nil
}

// EditPhoto replaces an existing photo message with a new image and caption.
func (b *Bot) EditPhoto(messageID int, imagePath, caption string, replyMarkup tgbotapi.InlineKeyboardMarkup) error {
	media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(imagePath))
	media.Caption = caption
	media.ParseMode = tgbotapi.ModeHTML

	edit := tgbotapi.EditMessageMediaConfig{
		BaseEdit: tgbotapi.BaseEdit{ChatID: b.chatID, MessageID: messageID, ReplyMarkup: &replyMarkup},
		Media:    media,
	}

	if _, err := b.api.Send(edit); err != nil {
		return fmt.Errorf("error editing photo message: %w", err)
	}

	return nil
}
