package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/git"
	"cotizaciones/internal/telegram"
	"cotizaciones/internal/ui"

	"github.com/joho/godotenv"
)

const (
	jsonOutputPath = "/opt/codes/cotizaciones_ng/docs/data.json"
	ngRepoPath     = "/opt/codes/cotizaciones_ng"
	totalSteps     = 8
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

	// 4. Telegram notification (non-fatal: skip on error)
	// - Spike (precio > promedio 7 días anteriores + 0.50): SIEMPRE envía mensaje NUEVO
	// - Diario normal: NO envía Telegram
	ui.StepStart(4, totalSteps, "📨", "Evaluando notificación de Telegram...")

	weeklyAvg, err := database.WeeklyAverage()
	if err != nil {
		ui.Warn(fmt.Sprintf("Error calculando promedio semanal: %v", err))
		weeklyAvg = 0
	}

	const spikeThreshold = 0.50
	isSpike := weeklyAvg > 0 && (data.Bid-weeklyAvg) > spikeThreshold

	if isSpike {
		diff := data.Bid - weeklyAvg
		ui.Info(fmt.Sprintf("🚨 SPIKE detectado: %.4f BOB (prom=%.4f, diff=+%.4f)", data.Bid, weeklyAvg, diff))

		cfg, err := database.GetConfig()
		if err != nil {
			ui.Warn(fmt.Sprintf("Error leyendo config, saltando Telegram: %v", err))
		} else {
			bot, err := telegram.New(token, cfg.ChatID)
			if err != nil {
				ui.Warn(fmt.Sprintf("Error creando bot de Telegram, saltando: %v", err))
			} else {
				ui.Success("Bot de Telegram conectado")
				today := time.Now().Format("2006-01-02")
				message := telegram.FormatSpikeMessage(data.Bid, weeklyAvg, diff)

				// Spike: siempre mensaje nuevo
				msgID, err := bot.SendMessage(message)
				if err != nil {
					ui.Warn(fmt.Sprintf("Error enviando alerta de spike: %v", err))
				} else {
					ui.Success(fmt.Sprintf("Alerta de spike enviada → msgID=%d", msgID))
					if err := database.UpdateConfig(today, strconv.Itoa(msgID)); err != nil {
						ui.Warn(fmt.Sprintf("Error actualizando config: %v", err))
					}
				}
			}
		}
	} else {
		if weeklyAvg > 0 {
			ui.Info(fmt.Sprintf("Sin spike (%.4f BOB, prom=%.4f, diff=+%.4f) — sin notificación",
				data.Bid, weeklyAvg, data.Bid-weeklyAvg))
		} else {
			ui.Info("Sin datos de semana anterior — sin notificación")
		}
	}

	// 5. Git pull forzado en el repo del frontend
	ui.StepStart(5, totalSteps, "🔄", "Actualizando repositorio (git pull forzado)...")
	if err := git.ForcePull(ngRepoPath); err != nil {
		exitWithError("Error en git pull: %v", err)
	}
	ui.Success(fmt.Sprintf("Repositorio actualizado → %s", ngRepoPath))

	// 6. Export all cotizaciones to JSON
	ui.StepStart(6, totalSteps, "📄", "Exportando cotizaciones a JSON...")
	if err := database.ExportCotizacionesToJSON(jsonOutputPath); err != nil {
		exitWithError("Error exportando JSON: %v", err)
	}
	ui.Success(fmt.Sprintf("Archivo generado → %s", jsonOutputPath))

	// 7. Git commit and push
	ui.StepStart(7, totalSteps, "🚀", "Subiendo cambios al repositorio (git push)...")
	commitMsg := "data upload"
	if err := git.CommitAndPush(ngRepoPath, commitMsg); err != nil {
		exitWithError("Error en git push: %v", err)
	}
	ui.Success("Cambios subidos correctamente")

	// 8. Cleanup old cotizaciones (older than 30 days)
	ui.StepStart(8, totalSteps, "🧹", "Limpiando registros antiguos (> 30 días)...")
	deleted, err := database.DeleteOlderThan(30 * 24 * time.Hour)
	if err != nil {
		exitWithError("Error limpiando registros: %v", err)
	}
	if deleted > 0 {
		ui.Success(fmt.Sprintf("Eliminados %d registros antiguos", deleted))
	} else {
		ui.Success("No hay registros antiguos para eliminar")
	}

	ui.Done()
}

// exitWithError prints a fatal error and terminates the process
func exitWithError(format string, args ...any) {
	ui.Fatal(fmt.Sprintf(format, args...))
	os.Exit(1)
}
