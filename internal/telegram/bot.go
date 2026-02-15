package telegram

import (
	"fmt"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram bot API
type Bot struct {
	api *tgbotapi.BotAPI
}

// New creates a new Telegram bot instance
func New(token string) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram bot: %w", err)
	}
	return &Bot{api: bot}, nil
}

// FormatMessage creates a nicely formatted HTML message for the cotizacion
func FormatMessage(bid float64) string {
	now := time.Now().Format("02/01/2006 15:04:05")
	return fmt.Sprintf(
		"<blockquote>💱 <b>Cotización del día</b></blockquote>\n\n"+
			"<b>💵 1 USDT = %.4f BOB</b>\n\n"+
			"<pre>"+
			"┌─────────────────────────┐\n"+
			"│  Moneda:    USDT        │\n"+
			"│  Precio:    %.4f BOB  │\n"+
			"│  Exchange:  Binance P2P │\n"+
			"└─────────────────────────┘"+
			"</pre>\n\n"+
			"🕐 <i>Actualizado: %s</i>",
		bid, bid, now,
	)
}

// SendMessage sends a new message to the specified chat
func (b *Bot) SendMessage(chatID string, text string) (int, error) {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	msg := tgbotapi.NewMessage(id, text)
	msg.ParseMode = tgbotapi.ModeHTML

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("error sending message: %w", err)
	}

	return sent.MessageID, nil
}

// EditMessage edits an existing message in the specified chat
func (b *Bot) EditMessage(chatID string, messageID string, text string) error {
	cid, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	mid, err := strconv.Atoi(messageID)
	if err != nil {
		return fmt.Errorf("invalid message ID %q: %w", messageID, err)
	}

	edit := tgbotapi.NewEditMessageText(cid, mid, text)
	edit.ParseMode = tgbotapi.ModeHTML

	_, err = b.api.Send(edit)
	if err != nil {
		return fmt.Errorf("error editing message: %w", err)
	}

	return nil
}
