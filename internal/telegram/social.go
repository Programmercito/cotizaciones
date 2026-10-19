package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cotizaciones/internal/db"
)

const socialDataPath = "/opt/osbo/cotiza/social.json"

type socialData struct {
	GeneratedAt string        `json:"generated_at"`
	Title       string        `json:"title"`
	Source      string        `json:"source"`
	Quotes      []socialQuote `json:"quotes"`
}

type socialQuote struct {
	Key       string       `json:"key"`
	Label     string       `json:"label"`
	Subtitle  string       `json:"subtitle,omitempty"`
	Sale      float64      `json:"sale"`
	Purchase  float64      `json:"purchase"`
	UpdatedAt string       `json:"updated_at"`
	Exchange  string       `json:"exchange,omitempty"`
	Currency  string       `json:"currency,omitempty"`
	Trend     *socialTrend `json:"trend,omitempty"`
}

type socialTrend struct {
	Direction string  `json:"direction"`
	Value     float64 `json:"value"`
}

type socialQuoteDefinition struct {
	key      string
	label    string
	subtitle string
}

var socialQuoteDefinitions = []socialQuoteDefinition{
	{key: "usdt", label: "USDT", subtitle: "Binance P2P"},
	{key: "usd_oficial", label: "USD Oficial", subtitle: "BCB"},
	{key: "usd_referencial", label: "USD Referencial", subtitle: "BCB"},
	{key: "euro", label: "Euro", subtitle: "BCB"},
	{key: "oro", label: "Oro", subtitle: "BCB"},
	{key: "plata", label: "Plata", subtitle: "BCB"},
	{key: "ufv", label: "UFV", subtitle: "BCB"},
}

// GenerateSocialData writes the latest quotes in a stable JSON format for the
// separate social-media video renderer.
func GenerateSocialData(summary map[string]db.Cotizacion, baselines db.USDBaselines) (string, error) {
	return GenerateSocialDataAt(socialDataPath, summary, baselines)
}

// GenerateSocialDataAt writes the video input JSON to outPath.
func GenerateSocialDataAt(outPath string, summary map[string]db.Cotizacion, baselines db.USDBaselines) (string, error) {
	data := socialData{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Title:       "Cotizaciones de Bolivia",
		Source:      "dolarbolivia.org",
		Quotes:      make([]socialQuote, 0, len(socialQuoteDefinitions)),
	}

	for _, definition := range socialQuoteDefinitions {
		dbKey := socialDBKey(definition.key)
		quote, ok := summary[dbKey]
		if !ok {
			continue
		}

		item := socialQuote{
			Key:       definition.key,
			Label:     definition.label,
			Subtitle:  definition.subtitle,
			Sale:      quote.Cotizacion,
			Purchase:  quote.Purchase,
			UpdatedAt: quote.Datetime,
			Exchange:  quote.Exchange,
			Currency:  quote.MonedaDest,
		}
		if trend := socialTrendFor(definition.key, quote.Cotizacion, baselines); trend != nil {
			item.Trend = trend
		}
		data.Quotes = append(data.Quotes, item)
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling social JSON: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("error creating social JSON directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outPath), ".social-*.json")
	if err != nil {
		return "", fmt.Errorf("error creating temporary social JSON: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := tempFile.Chmod(0644); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("error setting social JSON permissions: %w", err)
	}
	if _, err := tempFile.Write(encoded); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("error writing social JSON: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("error closing social JSON: %w", err)
	}
	if err := os.Rename(tempPath, outPath); err != nil {
		return "", fmt.Errorf("error replacing social JSON: %w", err)
	}

	return outPath, nil
}

func socialDBKey(key string) string {
	switch key {
	case "usdt":
		return "USDT"
	case "usd_oficial":
		return "usd oficial"
	case "usd_referencial":
		return "usd referencial"
	case "euro":
		return "eur"
	default:
		return key
	}
}

func socialTrendFor(key string, current float64, baselines db.USDBaselines) *socialTrend {
	var baseline db.Baseline
	switch key {
	case "usdt":
		baseline = baselines.USDT
	case "usd_oficial":
		baseline = baselines.UsdOficial
	default:
		return nil
	}
	if !baseline.Exists {
		return nil
	}

	diff := current - baseline.Value
	direction := "equal"
	if diff > 0.005 {
		direction = "up"
	} else if diff < -0.005 {
		direction = "down"
	}
	return &socialTrend{Direction: direction, Value: diff}
}
