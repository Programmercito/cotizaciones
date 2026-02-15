package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/telegram"
	"cotizaciones/internal/ui"

	"github.com/joho/godotenv"
)

const (
	jsonOutputPath = "/opt/osbo/codes/cotizaciones/dist/data.json"
	totalSteps     = 5
)

func main() {
	ui.Banner()

	if err := godotenv.Load(); err != nil {
		ui.Warn(".env no encontrado, usando variables de entorno del sistema")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		ui.Fatal("TELEGRAM_BOT_TOKEN es requerido")
		os.Exit(1)
	}

	// 1. Fetch cotizacion from API
	ui.StepStart(1, totalSteps, "🌐", "Consultando API de CriptoYa...")
	data, err := api.FetchCotizacion()
	if err != nil {
		exitWithError("Error consultando API: %v", err)
	}
	ui.Success("Respuesta recibida correctamente")
	ui.Price(data.Bid)

	// 2. Open database
	ui.StepStart(2, totalSteps, "🗄️", "Conectando a base de datos SQLite...")
	database, err := db.New()
	if err != nil {
		exitWithError("Error abriendo base de datos: %v", err)
	}
	defer database.Close()
	ui.Success("Conexión establecida")

	// 3. Insert cotizacion
	ui.StepStart(3, totalSteps, "💾", "Guardando cotización en base de datos...")
	if err := database.InsertCotizacion(data.Bid); err != nil {
		exitWithError("Error guardando cotización: %v", err)
	}
	ui.Success("Cotización guardada → moneda=USDT exchange=binancep2p")
	ui.Info(fmt.Sprintf("bid=%.4f  time=%s", data.Bid, time.Now().Format("2006-01-02 15:04:05")))

	// 4. Send/Edit Telegram message
	ui.StepStart(4, totalSteps, "📨", "Procesando notificación de Telegram...")

	cfg, err := database.GetConfig()
	if err != nil {
		exitWithError("Error leyendo config: %v", err)
	}

	bot, err := telegram.New(token, cfg.ChatID)
	if err != nil {
		exitWithError("Error creando bot de Telegram: %v", err)
	}
	ui.Success("Bot de Telegram conectado")

	today := time.Now().Format("2006-01-02")
	message := telegram.FormatMessage(data.Bid)

	msgID, err := sendOrEditMessage(bot, cfg, today, message)
	if err != nil {
		exitWithError("Error en notificación de Telegram: %v", err)
	}
	if err := database.UpdateConfig(today, strconv.Itoa(msgID)); err != nil {
		exitWithError("Error actualizando config: %v", err)
	}

	// 5. Export all cotizaciones to JSON
	ui.StepStart(5, totalSteps, "📄", "Exportando cotizaciones a JSON...")
	if err := database.ExportCotizacionesToJSON(jsonOutputPath); err != nil {
		exitWithError("Error exportando JSON: %v", err)
	}
	ui.Success(fmt.Sprintf("Archivo generado → %s", jsonOutputPath))

	ui.Done()
}

// sendOrEditMessage decides whether to edit an existing message or send a new one.
// Returns the final message ID to persist in config.
func sendOrEditMessage(bot *telegram.Bot, cfg *db.Config, today, message string) (int, error) {
	canEdit := cfg.CurrentDate == today && cfg.MessageID.Valid && cfg.MessageID.String != ""

	if canEdit {
		mid, _ := strconv.Atoi(cfg.MessageID.String)
		ui.Info("Fecha actual coincide, editando mensaje existente...")
		if err := bot.EditMessage(mid, message); err != nil {
			ui.Warn(fmt.Sprintf("No se pudo editar, enviando nuevo: %v", err))
		} else {
			ui.Success("Mensaje editado correctamente")
			return mid, nil
		}
	} else {
		ui.Info("Nueva fecha o sin mensaje previo, enviando mensaje nuevo...")
	}

	msgID, err := bot.SendMessage(message)
	if err != nil {
		return 0, err
	}
	ui.Success(fmt.Sprintf("Mensaje enviado → msgID=%d", msgID))
	return msgID, nil
}

// exitWithError prints a fatal error and terminates the process
func exitWithError(format string, args ...any) {
	ui.Fatal(fmt.Sprintf(format, args...))
	os.Exit(1)
}
