package telegram

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cotizaciones/internal/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestIsNotModified(t *testing.T) {
	if !isNotModified(errors.New("error editing message: Bad Request: message is not modified")) {
		t.Fatal("expected not-modified error to be detected")
	}
	if isNotModified(errors.New("Bad Request: chat not found")) {
		t.Fatal("unrelated error misclassified as not-modified")
	}
	if isNotModified(nil) {
		t.Fatal("nil must not be not-modified")
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rate limit", tgbotapi.Error{Code: 429, Message: "Too Many Requests"}, true},
		{"server 500", tgbotapi.Error{Code: 500, Message: "Internal Server Error"}, true},
		{"server 502", tgbotapi.Error{Code: 502, Message: "Bad Gateway"}, true},
		{"client 400", tgbotapi.Error{Code: 400, Message: "Bad Request: message to edit not found"}, false},
		{"not modified", tgbotapi.Error{Code: 400, Message: "Bad Request: message is not modified"}, false},
		{"network", errors.New("dial tcp: i/o timeout"), true},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryable_IdempotencyRules(t *testing.T) {
	networkErr := errors.New("context deadline exceeded")
	rateLimit := tgbotapi.Error{Code: 429, Message: "Too Many Requests"}
	serverErr := tgbotapi.Error{Code: 502, Message: "Bad Gateway"}

	// Edits are idempotent: safe to retry any transient failure.
	if !retryable(networkErr, true) {
		t.Error("edits should retry network errors")
	}
	if !retryable(serverErr, true) {
		t.Error("edits should retry 5xx")
	}
	// Sends are NOT idempotent: a network timeout might mean the message was
	// already delivered, so only an explicit 429 may be retried.
	if retryable(networkErr, false) {
		t.Error("sends must NOT retry network errors (duplicate risk)")
	}
	if retryable(serverErr, false) {
		t.Error("sends must NOT retry 5xx (duplicate risk)")
	}
	if !retryable(rateLimit, false) {
		t.Error("sends must retry 429 (Telegram guarantees it was not processed)")
	}
}

func TestRetryAfter(t *testing.T) {
	err := tgbotapi.Error{Code: 429, Message: "Too Many Requests", ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 37}}
	if got := retryAfter(err); got != 37*time.Second {
		t.Errorf("retryAfter = %v, want 37s", got)
	}
	if got := retryAfter(errors.New("plain")); got != 0 {
		t.Errorf("retryAfter on non-API error = %v, want 0", got)
	}
}

func TestChatGone(t *testing.T) {
	if !isChatGone(errors.New("Bad Request: chat not found")) {
		t.Error("chat not found not detected")
	}
	if !isChatGone(errors.New("Forbidden: bot is not a member of the chat")) {
		t.Error("bot kicked not detected")
	}
	// The exported predicate must work through the wrapping done in send().
	sentinel := fmt.Errorf("%w: Bad Request: chat not found", ErrChatNotFound)
	wrapped := fmt.Errorf("error sending message: %w", sentinel)
	if !IsChatGone(wrapped) {
		t.Error("IsChatGone must see ErrChatNotFound through %w wrapping")
	}
}

func TestVisibleLen(t *testing.T) {
	got := visibleLen("<b>hola</b> <i>mundo</i>")
	if got != len("hola mundo") {
		t.Errorf("visibleLen = %d, want %d", got, len("hola mundo"))
	}
}

func TestClampCaption(t *testing.T) {
	long := strings.Repeat("x", 900)
	text := "<b>titulo</b>\nline2\n" + long + "\nfooter <i>generado</i>"

	clamped := clampCaption(text, 900)
	if visibleLen(clamped) > 900 {
		t.Fatalf("clamped caption still exceeds limit: %d", visibleLen(clamped))
	}
	if !strings.HasPrefix(clamped, "<b>titulo</b>") {
		t.Error("clamp must keep the first line intact")
	}
	if strings.Contains(clamped, "footer") || strings.Contains(clamped, long) {
		t.Error("clamp should drop trailing lines until the caption fits")
	}

	// Under-limit text must come back untouched.
	if clampCaption("short <b>caption</b>", 1024) != "short <b>caption</b>" {
		t.Error("short caption must not be modified")
	}
}

func TestFmtDestEscapesHTML(t *testing.T) {
	if got := fmtDest("A&B <x>"); got != " (A&amp;B &lt;x&gt;)" {
		t.Errorf("fmtDest escaping = %q", got)
	}
	if fmtDest("") != "" {
		t.Error("empty dest must render empty")
	}
}

func TestQuoteRowOmitsCompraAndTrendWhenAbsent(t *testing.T) {
	c := db.Cotizacion{Cotizacion: 6.98, Purchase: 0, Datetime: "2026-10-24 14:32:05"}
	joined := strings.Join(quoteRow("💰", "USDT · Binance P2P", "Venta", c, db.Baseline{}), "\n")

	if strings.Contains(joined, "Compra") {
		t.Error("must hide Compra 0.00 noise for quotes without purchase data")
	}
	if strings.Contains(joined, "vs ayer") {
		t.Error("must hide trend when no baseline exists")
	}
	if !strings.Contains(joined, "14:32") || !strings.Contains(joined, "6.98") {
		t.Errorf("row content wrong: %q", joined)
	}
}

func TestQuoteRowShowsCompraAndTrend(t *testing.T) {
	c := db.Cotizacion{Cotizacion: 6.98, Purchase: 7.02, Datetime: "2026-10-24 14:32:05"}
	base := db.Baseline{Value: 6.91, Exists: true}
	joined := strings.Join(quoteRow("💰", "USDT · Binance P2P", "Venta", c, base), "\n")

	if !strings.Contains(joined, "Compra <code>7.02</code>") {
		t.Error("missing compra when purchase > 0")
	}
	if !strings.Contains(joined, "▲") || !strings.Contains(joined, "+0.07") || !strings.Contains(joined, "1.0%") {
		t.Errorf("expected up-trend with diff and pct, got %q", joined)
	}
	if !strings.Contains(joined, "vs ayer") {
		t.Error("trend must be labelled vs ayer")
	}
}

func TestTrendTextEqualAvoidsZeroNoise(t *testing.T) {
	tr := buildTrend(7.0, db.Baseline{Value: 7.0, Exists: true})
	got := trendText(tr, 7.0)
	if !strings.Contains(got, "sin cambio") {
		t.Errorf("equal trend = %q, want 'sin cambio'", got)
	}
	if strings.Contains(got, "+0.00") {
		t.Errorf("equal trend shows misleading +0.00: %q", got)
	}
}

func TestTrendTextDown(t *testing.T) {
	tr := buildTrend(6.80, db.Baseline{Value: 7.00, Exists: true})
	got := trendText(tr, 7.00)
	if !strings.Contains(got, "▼") || !strings.Contains(got, "-0.20") || !strings.Contains(got, "2.9%") {
		t.Errorf("down trend = %q", got)
	}
}

func TestShortTimeDateOnly(t *testing.T) {
	if got := shortTime("2026-01-02"); got != "02/01" {
		t.Errorf("date-only = %q, want 02/01", got)
	}
	if got := shortTime("basura"); got != "" {
		t.Errorf("garbage must render empty, got %q", got)
	}
}
