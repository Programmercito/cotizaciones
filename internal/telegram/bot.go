package telegram

import (
	"cotizaciones/internal/db"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const siteURL = "https://cotizaciones.devcito.org/"

// fmtDT convierte un datetime de la DB al formato legible para Telegram.
// Si el valor contiene hora, muestra segundos; si es solo fecha, muestra solo la fecha.
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

	return dt
}

// fmtDest returns a formatted moneda destino tag, or empty if blank.
func fmtDest(dest string) string {
	if dest == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", dest)
}

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
	oficial := summary["usd oficial"]
	referencial := summary["usd referencial"]
	euro := summary["eur"]
	oro := summary["oro"]
	plata := summary["plata"]
	ufv := summary["ufv"]
	pct := (math.Abs(diff) / umbral) * 100
	generatedAt := time.Now().Format(db.DisplayTimeFmt)

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
		"💰 <b>USDT (Binance):</b>" + fmtDest(usdt.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.4f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.4f</code>", usdt.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(usdt.Datetime)),
		"",
		"🏢 <b>BCB - USD Oficial:</b>" + fmtDest(oficial.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", oficial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", oficial.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(oficial.Datetime)),
		"",
		"📊 <b>BCB - USD Referencial:</b>" + fmtDest(referencial.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", referencial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", referencial.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(referencial.Datetime)),
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
		fmt.Sprintf("💵 Valor:  <code>%.5f</code>", ufv.Cotizacion),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(ufv.Datetime)),
		"────────────────────────",
		fmt.Sprintf("📊 Variación USDT: <code>%s%.4f</code> (<code>%s%.2f%%</code>)", dir, math.Abs(diff), dir, pct),
		fmt.Sprintf("🏷️ Ref. Anterior: <code>%.4f</code>", umbral),
		fmt.Sprintf("📅 <i>Generado: %s</i>", generatedAt),
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
	usdt := summary["USDT"]
	oficial := summary["usd oficial"]
	referencial := summary["usd referencial"]
	euro := summary["eur"]
	oro := summary["oro"]
	plata := summary["plata"]
	ufv := summary["ufv"]
	generatedAt := time.Now().Format(db.DisplayTimeFmt)

	text := strings.Join([]string{
		"<blockquote><b>☀️ Resumen de Cotizaciones</b></blockquote>",
		"🏛️ <b>Mercados:</b> Binance P2P / BCB",
		"",
		"💰 <b>USDT (Binance):</b>" + fmtDest(usdt.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.4f</code>", usdt.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.4f</code>", usdt.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(usdt.Datetime)),
		"",
		"🏢 <b>BCB - USD Oficial:</b>" + fmtDest(oficial.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", oficial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", oficial.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(oficial.Datetime)),
		"",
		"📊 <b>BCB - USD Referencial:</b>" + fmtDest(referencial.MonedaDest),
		fmt.Sprintf("💵 Venta:  <code>%.2f</code>", referencial.Cotizacion),
		fmt.Sprintf("🛒 Compra: <code>%.2f</code>", referencial.Purchase),
		fmt.Sprintf("🕒 <i>%s</i>", fmtDT(referencial.Datetime)),
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
		fmt.Sprintf("💵 Valor:  <code>%.5f</code>", ufv.Cotizacion),
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
