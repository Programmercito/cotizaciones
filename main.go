package main

import (
	"fmt"
	"os"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/telegram"
	"cotizaciones/internal/ui"

	"github.com/joho/godotenv"
)

const jsonOutputPath = "/opt/osbo/codes/cotizaciones/dist/data.json"

func main() {
	ui.Banner()

	// Load .env file
	if err := godotenv.Load(); err != nil {
		ui.Warn(".env no encontrado, usando variables de entorno del sistema")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		ui.Fatal("TELEGRAM_BOT_TOKEN es requerido")
		os.Exit(1)
	}

	// 1. Fetch cotizacion from API
	ui.StepStart(1, "🌐", "Consultando API de CriptoYa...")
	data, err := api.FetchCotizacion()
	if err != nil {
		ui.Fatal(fmt.Sprintf("Error consultando API: %v", err))
		os.Exit(1)
	}
	ui.Success("Respuesta recibida correctamente")
	ui.Price(data.Bid)

	// 2. Open database
	ui.StepStart(2, "🗄️", "Conectando a base de datos SQLite...")
	database, err := db.New()
	if err != nil {
		ui.Fatal(fmt.Sprintf("Error abriendo base de datos: %v", err))
		os.Exit(1)
	}
	defer database.Close()
	ui.Success("Conexión establecida")

	// 3. Insert cotizacion into database
	ui.StepStart(3, "💾", "Guardando cotización en base de datos...")
	if err := database.InsertCotizacion(data.Bid); err != nil {
		ui.Fatal(fmt.Sprintf("Error guardando cotización: %v", err))
		os.Exit(1)
	}
	ui.Success("Cotización guardada → moneda=USDT exchange=binancep2p")
	ui.Info(fmt.Sprintf("bid=%.4f  time=%s",
		data.Bid,
		time.Now().Format("2006-01-02 15:04:05"),
	))

	// 4. Send/Edit Telegram message
	ui.StepStart(4, "📨", "Procesando notificación de Telegram...")
	bot, err := telegram.New(token)
	if err != nil {
		ui.Fatal(fmt.Sprintf("Error creando bot de Telegram: %v", err))
		os.Exit(1)
	}
	ui.Success("Bot de Telegram conectado")

	cfg, err := database.GetConfig()
	if err != nil {
		ui.Fatal(fmt.Sprintf("Error leyendo config: %v", err))
		os.Exit(1)
	}

	today := time.Now().Format("2006-01-02")
	message := telegram.FormatMessage(data.Bid)

	if cfg.CurrentDate == today && cfg.MessageID.Valid && cfg.MessageID.String != "" {
		ui.Info("Fecha actual coincide, editando mensaje existente...")
		if err := bot.EditMessage(cfg.ChatID, cfg.MessageID.String, message); err != nil {
			ui.Warn(fmt.Sprintf("No se pudo editar, enviando nuevo: %v", err))
			msgID, err := bot.SendMessage(cfg.ChatID, message)
			if err != nil {
				ui.Fatal(fmt.Sprintf("Error enviando mensaje: %v", err))
				os.Exit(1)
			}
			if err := database.UpdateConfig(today, fmt.Sprintf("%d", msgID)); err != nil {
				ui.Fatal(fmt.Sprintf("Error actualizando config: %v", err))
				os.Exit(1)
			}
			ui.Success(fmt.Sprintf("Nuevo mensaje enviado → msgID=%d", msgID))
		} else {
			ui.Success("Mensaje editado correctamente")
		}
	} else {
		ui.Info("Nueva fecha o sin mensaje previo, enviando mensaje nuevo...")
		msgID, err := bot.SendMessage(cfg.ChatID, message)
		if err != nil {
			ui.Fatal(fmt.Sprintf("Error enviando mensaje: %v", err))
			os.Exit(1)
		}
		if err := database.UpdateConfig(today, fmt.Sprintf("%d", msgID)); err != nil {
			ui.Fatal(fmt.Sprintf("Error actualizando config: %v", err))
			os.Exit(1)
		}
		ui.Success(fmt.Sprintf("Mensaje enviado → msgID=%d", msgID))
	}

	// 5. Export all cotizaciones to JSON
	ui.StepStart(5, "📄", "Exportando cotizaciones a JSON...")
	if err := database.ExportCotizacionesToJSON(jsonOutputPath); err != nil {
		ui.Fatal(fmt.Sprintf("Error exportando JSON: %v", err))
		os.Exit(1)
	}
	ui.Success(fmt.Sprintf("Archivo generado → %s", jsonOutputPath))

	ui.Done()
}
