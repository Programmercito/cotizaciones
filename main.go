package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/git"
	"cotizaciones/internal/telegram"
	"cotizaciones/internal/ui"

	"github.com/joho/godotenv"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	ui.Prices(data.Bid, data.TotalAsk)

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
	if err := database.InsertCotizacion(data.Bid, data.TotalAsk); err != nil {
		exitWithError("Error guardando cotización: %v", err)
	}
	ui.Success("Cotización guardada → moneda=USDT exchange=binancep2p")
	ui.Info(fmt.Sprintf("bid=%.2f  purchase=%.2f  time=%s", data.Bid, data.TotalAsk, time.Now().Format("2006-01-02 15:04:05")))

	// 4. Telegram (non-fatal: errores no cortan el flujo)
	ui.StepStart(4, totalSteps, "📨", "Procesando notificación de Telegram...")

	summary, err := database.GetLatestSummary()
	if err != nil {
		ui.Warn(fmt.Sprintf("Error obteniendo resumen para Telegram: %v", err))
	}

	imagePath, imageErr := telegram.GeneratePriceImage(summary)
	if imageErr != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar la imagen de cotización: %v", imageErr))
	}
	if imagePath != "" {
		defer os.Remove(imagePath)
	}

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

			const spikeThreshold = 0.50
			hasMessage := cfg.MessageID.Valid && cfg.MessageID.String != ""

			usdRef := summary["usd referencial"]

			// umbral USDT (cfg.Umbral): referencia para calcular spike de USDT
			// Se inicializa con data.Bid la primera vez, luego solo cambia si hay spike.
			usdtUmbralNull := !cfg.Umbral.Valid
			currentUmbralUSDT := data.Bid
			if !usdtUmbralNull {
				currentUmbralUSDT = cfg.Umbral.Float64
			}

			// umbral USD Referencial (cfg.UmbralReferencial): referencia para spike de USD Ref
			refUmbralNull := !cfg.UmbralReferencial.Valid
			currentUmbralRef := usdRef.Cotizacion
			if !refUmbralNull {
				currentUmbralRef = cfg.UmbralReferencial.Float64
			}

			diffUSDT := data.Bid - currentUmbralUSDT
			diffRef := usdRef.Cotizacion - currentUmbralRef

			spikeUSDT := !usdtUmbralNull && math.Abs(diffUSDT) >= spikeThreshold
			spikeRef := !refUmbralNull && math.Abs(diffRef) >= spikeThreshold
			isSpike := spikeUSDT || spikeRef

			// diff principal para el mensaje de spike (el mayor)
			diff := diffUSDT
			if math.Abs(diffRef) > math.Abs(diffUSDT) {
				diff = diffRef
			}

			// tryS: envía foto si existe; si falla cae a texto
			tryS := func(text string, silent bool, btn tgbotapi.InlineKeyboardMarkup) (int, error) {
				if imagePath != "" {
					id, e := bot.SendPhoto(imagePath, text, silent, btn)
					if e == nil {
						return id, nil
					}
					ui.Warn(fmt.Sprintf("Foto falló (%v), enviando texto...", e))
				}
				return bot.SendMessage(text, silent, btn)
			}

			// saveConfig: guarda messageID y ambos umbrales con los precios actuales
			saveConfig := func(msgID string) {
				if err := database.UpdateConfig(today, msgID, data.Bid, usdRef.Cotizacion); err != nil {
					ui.Warn(fmt.Sprintf("Error guardando config: %v", err))
				}
			}

			switch {

			case !hasMessage:
				// REGLA 1: No hay messageID → enviar nuevo y guardarlo sí o sí
				ui.Info("Sin messageID — enviando mensaje nuevo...")
				msg, btn := telegram.FormatDailyMessage(summary)
				newID, e := tryS(msg, true, btn)
				if e != nil {
					ui.Warn(fmt.Sprintf("Error enviando mensaje: %v", e))
				} else {
					ui.Success(fmt.Sprintf("Mensaje enviado → msgID=%d", newID))
					saveConfig(strconv.Itoa(newID))
				}

			case isSpike:
				// REGLA 3: Spike (±0.50) → enviar mensaje NUEVO, actualizar messageID y umbrales
				ui.Info(fmt.Sprintf("🚨 SPIKE: USDT=%.4f(dif=%+.4f) Ref=%.4f(dif=%+.4f)",
					data.Bid, diffUSDT, usdRef.Cotizacion, diffRef))
				msg, btn := telegram.FormatSpikeMessage(summary, currentUmbralUSDT, diff, diff > 0)
				newID, e := tryS(msg, false, btn)
				if e != nil {
					ui.Warn(fmt.Sprintf("Error enviando spike: %v", e))
				} else {
					ui.Success(fmt.Sprintf("Spike enviado → nuevo msgID=%d", newID))
					saveConfig(strconv.Itoa(newID))
				}

			default:
				// REGLA 2: Hay messageID, sin spike → editar mensaje existente
				mid, _ := strconv.Atoi(cfg.MessageID.String)
				ui.Info(fmt.Sprintf("Actualizando mensaje existente (id=%d)...", mid))
				msg, btn := telegram.FormatDailyMessage(summary)
				var editErr error
				if imagePath != "" {
					editErr = bot.EditPhoto(mid, imagePath, msg, btn)
				} else {
					editErr = bot.EditMessage(mid, msg, btn)
				}
				if editErr != nil {
					// Mensaje borrado o inaccesible → enviar uno nuevo
					ui.Warn(fmt.Sprintf("No se pudo editar (%v) — enviando nuevo...", editErr))
					newID, e := tryS(msg, true, btn)
					if e != nil {
						ui.Warn(fmt.Sprintf("Error enviando fallback: %v", e))
					} else {
						ui.Success(fmt.Sprintf("Nuevo mensaje enviado → msgID=%d", newID))
						saveConfig(strconv.Itoa(newID))
					}
				} else {
					ui.Success("Mensaje actualizado correctamente")
					saveConfig(strconv.Itoa(mid))
				}
			}
		}
	}

	// 5. Git pull forzado en el repo del frontend
	ui.StepStart(5, totalSteps, "🔄", "Actualizando repositorio (git pull forzado)...")
	if err := git.ForcePull(ngRepoPath); err != nil {
		exitWithError("Error en git pull: %v", err)
	}
	ui.Success(fmt.Sprintf("Repositorio actualizado → %s", ngRepoPath))

	// 5. Export all cotizaciones to JSON
	ui.StepStart(5, totalSteps-1, "📄", "Exportando cotizaciones a JSON...")
	if err := database.ExportCotizacionesToJSON(jsonOutputPath); err != nil {
		exitWithError("Error exportando JSON: %v", err)
	}
	ui.Success(fmt.Sprintf("Archivo generado → %s", jsonOutputPath))

	// 6. Git commit and push
	ui.StepStart(6, totalSteps-1, "🚀", "Subiendo cambios al repositorio (git push)...")
	commitMsg := "data upload"
	if err := git.CommitAndPush(ngRepoPath, commitMsg); err != nil {
		exitWithError("Error en git push: %v", err)
	}
	ui.Success("Cambios subidos correctamente")

	// 7. Cleanup old cotizaciones (older than 30 days)
	ui.StepStart(7, totalSteps-1, "🧹", "Limpiando registros antiguos (> 30 días)...")
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
