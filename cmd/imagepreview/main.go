package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"cotizaciones/internal/db"
	"cotizaciones/internal/telegram"
)

func main() {
	now := time.Now().Format(db.TimeFmt)
	summary := map[string]db.Cotizacion{
		"USDT": {
			Moneda:     "USDT",
			Cotizacion: 7.18,
			Purchase:   7.12,
			Datetime:   now,
			Exchange:   "binancep2p",
			MonedaDest: "BOB",
		},
		"usd oficial": {
			Moneda:     "usd oficial",
			Cotizacion: 6.86,
			Purchase:   6.76,
			Datetime:   now,
			Exchange:   "bcb",
			MonedaDest: "BOB",
		},
	}
	baselines := db.USDBaselines{
		USDT:       db.Baseline{Value: 7.02, Exists: true},
		UsdOficial: db.Baseline{Value: 6.95, Exists: true},
	}

	output := filepath.Join("docs", "usd-preview.png")
	path, err := telegram.GenerateUSDImageAt(output, summary, false, baselines)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(path)
}
