package telegram

import (
	"fmt"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram bot API
type Bot struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

// New creates a new Telegram bot instance bound to a specific chatID
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

// SendMessage sends a new message and returns the message ID
func (b *Bot) SendMessage(text string) (int, error) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("error sending message: %w", err)
	}

	return sent.MessageID, nil
}

// EditMessage edits an existing message by its ID
func (b *Bot) EditMessage(messageID int, text string) error {
	edit := tgbotapi.NewEditMessageText(b.chatID, messageID, text)
	edit.ParseMode = tgbotapi.ModeHTML

	if _, err := b.api.Send(edit); err != nil {
		return fmt.Errorf("error editing message: %w", err)
	}

	return nil
}
