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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

	showReferencial := telegram.ShowReferencial(time.Now())

	// Generate images for both messages
	imagePathUSD, imageErrUSD := telegram.GenerateUSDImage(summary, showReferencial)
	if imageErrUSD != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar la imagen USD: %v", imageErrUSD))
	}

	imagePathResto, imageErrResto := telegram.GenerateRestoImage(summary)
	if imageErrResto != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar la imagen Resto: %v", imageErrResto))
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
			const spikeThreshold = 0.20

			// Generic helpers
			trySend := func(imgPath, text string, silent bool, btn tgbotapi.InlineKeyboardMarkup) (int, error) {
				if imgPath != "" {
					id, e := bot.SendPhoto(imgPath, text, silent, btn)
					if e == nil {
						return id, nil
					}
					ui.Warn(fmt.Sprintf("Foto falló (%v), enviando texto...", e))
				}
				return bot.SendMessage(text, silent, btn)
			}

			editMsg := func(imgPath string, mid int, text string, btn tgbotapi.InlineKeyboardMarkup) error {
				if imgPath != "" {
					return bot.EditPhoto(mid, imgPath, text, btn)
				}
				return bot.EditMessage(mid, text, btn)
			}

			// Threshold calculations must happen before any message is sent,
			// because a USD spike forces both messages to be re-sent together.
			usdRef := summary["usd referencial"]

			usdtUmbralNull := !cfg.Umbral.Valid
			currentUmbralUSDT := data.Bid
			if !usdtUmbralNull {
				currentUmbralUSDT = cfg.Umbral.Float64
			}

			refUmbralNull := !cfg.UmbralReferencial.Valid
			currentUmbralRef := usdRef.Cotizacion
			if !refUmbralNull {
				currentUmbralRef = cfg.UmbralReferencial.Float64
			}

			hasUmbrales := !usdtUmbralNull && !refUmbralNull

			var diffUSDT, diffRef, diff float64
			var isOutside bool
			if hasUmbrales {
				diffUSDT = data.Bid - currentUmbralUSDT
				diffRef = usdRef.Cotizacion - currentUmbralRef
				outsideUSDT := math.Abs(diffUSDT) > spikeThreshold
				outsideRef := math.Abs(diffRef) > spikeThreshold
				isOutside = outsideUSDT || outsideRef

				diff = diffUSDT
				if math.Abs(diffRef) > math.Abs(diffUSDT) {
					diff = diffRef
				}
			}

			hasMessageResto := cfg.MessageID.Valid && cfg.MessageID.String != ""
			hasMessageUSD := cfg.MessageIDUSD.Valid && cfg.MessageIDUSD.String != ""

			saveMessageIDResto := func(msgID string) {
				if err := database.UpdateConfigMessageID(today, msgID); err != nil {
					ui.Warn(fmt.Sprintf("Error guardando messageID: %v", err))
				}
			}

			saveMessageIDUSD := func(msgID string) {
				if err := database.UpdateConfigMessageIDUSD(today, msgID); err != nil {
					ui.Warn(fmt.Sprintf("Error guardando messageIDUSD: %v", err))
				}
			}

			saveConfigUSD := func(msgID string) {
				if err := database.UpdateConfigUSD(today, msgID, data.Bid, usdRef.Cotizacion); err != nil {
					ui.Warn(fmt.Sprintf("Error guardando config USD: %v", err))
				}
			}

			sendResto := func() (int, error) {
				msg, btn := telegram.FormatRestoMessage(summary)
				return trySend(imagePathResto, msg, true, btn)
			}

		sendUSD := func() (int, error) {
			msg, btn := telegram.FormatUSDMessage(summary, showReferencial)
			return trySend(imagePathUSD, msg, true, btn)
		}

		sendSpikeUSD := func() (int, error) {
			msg, btn := telegram.FormatSpikeUSDMessage(summary, currentUmbralUSDT, diffUSDT, currentUmbralRef, diffRef, showReferencial)
			return trySend(imagePathUSD, msg, false, btn)
		}

			editResto := func(mid int) error {
				msg, btn := telegram.FormatRestoMessage(summary)
				return editMsg(imagePathResto, mid, msg, btn)
			}

		editUSD := func(mid int) error {
			msg, btn := telegram.FormatUSDMessage(summary, showReferencial)
			return editMsg(imagePathUSD, mid, msg, btn)
		}

			switch {
			case !hasUmbrales:
				ui.Info("Sin umbrales definidos — guardando referencias y omitiendo notificación USD.")
				saveConfigUSD(cfg.MessageIDUSD.String)

				if hasMessageResto {
					mid, _ := strconv.Atoi(cfg.MessageID.String)
					ui.Info(fmt.Sprintf("Actualizando mensaje Resto existente (id=%d)...", mid))
					if editErr := editResto(mid); editErr != nil {
						ui.Warn(fmt.Sprintf("No se pudo editar Resto (%v) — enviando nuevo...", editErr))
						newID, e := sendResto()
						if e != nil {
							ui.Warn(fmt.Sprintf("Error enviando fallback Resto: %v", e))
						} else {
							ui.Success(fmt.Sprintf("Nuevo mensaje Resto enviado → msgID=%d", newID))
							saveMessageIDResto(strconv.Itoa(newID))
						}
					} else {
						ui.Success("Mensaje Resto actualizado correctamente")
						saveMessageIDResto(strconv.Itoa(mid))
					}
				} else {
					ui.Info("Sin messageID — enviando mensaje Resto nuevo...")
					newID, e := sendResto()
					if e != nil {
						ui.Warn(fmt.Sprintf("Error enviando mensaje Resto: %v", e))
					} else {
						ui.Success(fmt.Sprintf("Mensaje Resto enviado → msgID=%d", newID))
						saveMessageIDResto(strconv.Itoa(newID))
					}
				}

			case isOutside:
				ui.Info(fmt.Sprintf("🚨 Fuera del umbral USD: USDT=%.2f(dif=%+.2f) Ref=%.2f(dif=%+.2f) — reenviando ambos mensajes para mantenerlos juntos",
					data.Bid, diffUSDT, usdRef.Cotizacion, diffRef))

				newRestoID, errResto := sendResto()
				if errResto != nil {
					ui.Warn(fmt.Sprintf("Error enviando mensaje Resto durante spike: %v", errResto))
				} else {
					ui.Success(fmt.Sprintf("Mensaje Resto reenviado → msgID=%d", newRestoID))
					saveMessageIDResto(strconv.Itoa(newRestoID))
				}

				newUSDID, errUSD := sendSpikeUSD()
				if errUSD != nil {
					ui.Warn(fmt.Sprintf("Error enviando spike USD: %v", errUSD))
				} else {
					ui.Success(fmt.Sprintf("Spike USD enviado → nuevo msgID=%d", newUSDID))
					saveConfigUSD(strconv.Itoa(newUSDID))
				}

			default:
				// No spike: update existing messages when possible.
				if hasMessageResto {
					mid, _ := strconv.Atoi(cfg.MessageID.String)
					ui.Info(fmt.Sprintf("Actualizando mensaje Resto existente (id=%d)...", mid))
					if editErr := editResto(mid); editErr != nil {
						ui.Warn(fmt.Sprintf("No se pudo editar Resto (%v) — enviando nuevo...", editErr))
						newID, e := sendResto()
						if e != nil {
							ui.Warn(fmt.Sprintf("Error enviando fallback Resto: %v", e))
						} else {
							ui.Success(fmt.Sprintf("Nuevo mensaje Resto enviado → msgID=%d", newID))
							saveMessageIDResto(strconv.Itoa(newID))
						}
					} else {
						ui.Success("Mensaje Resto actualizado correctamente")
						saveMessageIDResto(strconv.Itoa(mid))
					}
				} else {
					ui.Info("Sin messageID — enviando mensaje Resto nuevo...")
					newID, e := sendResto()
					if e != nil {
						ui.Warn(fmt.Sprintf("Error enviando mensaje Resto: %v", e))
					} else {
						ui.Success(fmt.Sprintf("Mensaje Resto enviado → msgID=%d", newID))
						saveMessageIDResto(strconv.Itoa(newID))
					}
				}

				if hasMessageUSD {
					mid, _ := strconv.Atoi(cfg.MessageIDUSD.String)
					ui.Info(fmt.Sprintf("Actualizando mensaje USD existente (id=%d)...", mid))
					if editErr := editUSD(mid); editErr != nil {
						ui.Warn(fmt.Sprintf("No se pudo editar USD (%v) — enviando nuevo...", editErr))
						newID, e := sendUSD()
						if e != nil {
							ui.Warn(fmt.Sprintf("Error enviando fallback USD: %v", e))
						} else {
							ui.Success(fmt.Sprintf("Nuevo mensaje USD enviado → msgID=%d", newID))
							saveMessageIDUSD(strconv.Itoa(newID))
						}
					} else {
						ui.Success("Mensaje USD actualizado correctamente")
						saveMessageIDUSD(strconv.Itoa(mid))
					}
				} else {
					ui.Info("Sin messageIDUSD — enviando mensaje USD nuevo...")
					newID, e := sendUSD()
					if e != nil {
						ui.Warn(fmt.Sprintf("Error enviando mensaje USD: %v", e))
					} else {
						ui.Success(fmt.Sprintf("Mensaje USD enviado → msgID=%d", newID))
						saveMessageIDUSD(strconv.Itoa(newID))
					}
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

	// 7. Cleanup old cotizaciones (older than 60 days)
	ui.StepStart(7, totalSteps-1, "🧹", "Limpiando registros antiguos (> 60 días)...")
	deleted, err := database.DeleteOlderThan(2 * 30 * 24 * time.Hour)
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
