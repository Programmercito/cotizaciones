package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/git"
	"cotizaciones/internal/lock"
	"cotizaciones/internal/telegram"
	"cotizaciones/internal/ui"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const (
	jsonOutputPath = "/opt/codes/cotizaciones_ng/docs/data.json"
	ngRepoPath     = "/opt/codes/cotizaciones_ng"
	lockFilePath   = "/opt/osbo/cotizaciones.lock"
	totalSteps     = 8
)

func main() {
	ui.Banner()

	if err := godotenv.Load(); err != nil {
		ui.Warn(".env no encontrado, usando variables de entorno del sistema")
	}

	// Prevent overlapping runs: a stuck previous execution sharing the fixed
	// image paths or editing the same messages corrupts Telegram state and
	// produces duplicate messages. Exit quietly; cron will run us again.
	releaseLock, lockErr := lock.TryAcquire(lockFilePath)
	if lockErr != nil {
		ui.Warn(fmt.Sprintf("Otra ejecución está en curso (%v) — salteando esta corrida.", lockErr))
		return
	}
	defer releaseLock()

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

	summary, summaryErr := database.GetLatestSummary()
	if summaryErr != nil {
		ui.Warn(fmt.Sprintf("Error obteniendo resumen para Telegram: %v", summaryErr))
	}

	showReferencial := telegram.ShowReferencial(time.Now())

	baselines, baselineErr := database.GetUSDBaselines()
	if baselineErr != nil {
		ui.Warn(fmt.Sprintf("Error obteniendo referencias del día anterior: %v", baselineErr))
	}

	socialDataPath, socialDataErr := telegram.GenerateSocialData(summary, baselines)
	if socialDataErr != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar el JSON para redes: %v", socialDataErr))
	} else {
		ui.Success(fmt.Sprintf("JSON para redes generado → %s", socialDataPath))
	}

	usdImagePath := ""
	if path, imageErr := telegram.GenerateUSDImage(summary, showReferencial, baselines); imageErr != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar la imagen USD: %v", imageErr))
	} else {
		usdImagePath = path
		ui.Success(fmt.Sprintf("Imagen USD generada → %s", path))
	}

	restoImagePath := ""
	if path, imageErr := telegram.GenerateRestoImage(summary); imageErr != nil {
		ui.Warn(fmt.Sprintf("No se pudo generar la imagen Resto: %v", imageErr))
	} else {
		restoImagePath = path
		ui.Success(fmt.Sprintf("Imagen Resto generada → %s", path))
	}

	cfg, err := database.GetConfig()
	if err != nil {
		ui.Warn(fmt.Sprintf("Error leyendo config, saltando Telegram: %v", err))
	} else if summaryErr != nil {
		ui.Warn("Resumen incompleto, saltando Telegram para no publicar datos inválidos.")
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
					if info, statErr := os.Stat(imgPath); statErr != nil {
						ui.Warn(fmt.Sprintf("No se puede leer la imagen %q: %v", imgPath, statErr))
					} else if info.Size() == 0 {
						ui.Warn(fmt.Sprintf("La imagen %q está vacía", imgPath))
					} else {
						id, e := bot.SendPhoto(imgPath, text, silent, btn)
						if e == nil {
							return id, nil
						}
						if telegram.IsChatGone(e) {
							exitWithError("El bot ya no puede escribir en el chat (%v) — revisá chatid/miembros. Saliendo con error para que el CronJob registre el fallo.", e)
						}
						ui.Warn(fmt.Sprintf("Telegram rechazó la foto %q (%v), enviando texto...", imgPath, e))
					}
				}
				id, e := bot.SendMessage(text, silent, btn)
				if e != nil && telegram.IsChatGone(e) {
					exitWithError("El bot ya no puede escribir en el chat (%v) — revisá chatid/miembros. Saliendo con error para que el CronJob registre el fallo.", e)
				}
				return id, e
			}

			// editMsg edits an existing message without knowing its type
			// (photo or text) up front. It tries the operations in a chain so a
			// one-off photo fallback can no longer poison every later run:
			//   - replace photo+caption (when an image exists)
			//   - edit text (works on text messages)
			//   - edit caption only (works on photo messages when no image)
			// "message is not modified" and transient errors are already handled
			// inside the bot methods; only a real failure here sends a new msg.
			editMsg := func(imgPath string, mid int, text string, btn tgbotapi.InlineKeyboardMarkup) error {
				var errs []string
				var lastErr error
				if imgPath != "" {
					if err := bot.EditPhoto(mid, imgPath, text, btn); err == nil {
						return nil
					} else {
						errs = append(errs, fmt.Sprintf("photo: %v", err))
						lastErr = err
						if telegram.IsChatGone(err) {
							exitWithError("El bot ya no puede escribir en el chat (%v) — revisá chatid/miembros. Saliendo con error para que el CronJob registre el fallo.", err)
						}
					}
				}
				if err := bot.EditMessage(mid, text, btn); err == nil {
					return nil
				} else {
					errs = append(errs, fmt.Sprintf("text: %v", err))
					lastErr = err
					if telegram.IsChatGone(err) {
						exitWithError("El bot ya no puede escribir en el chat (%v) — revisá chatid/miembros. Saliendo con error para que el CronJob registre el fallo.", err)
					}
				}
				if err := bot.EditCaption(mid, text, btn); err == nil {
					return nil
				} else {
					errs = append(errs, fmt.Sprintf("caption: %v", err))
					lastErr = err
				}
				return fmt.Errorf("todas las ediciones fallaron (msgID=%d) — %s: %w", mid, strings.Join(errs, " | "), lastErr)
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

			hasUmbrales := !usdtUmbralNull && (!showReferencial || !refUmbralNull)

			var diffUSDT, diffRef float64
			var isOutside bool
			if hasUmbrales {
				diffUSDT = data.Bid - currentUmbralUSDT
				diffRef = usdRef.Cotizacion - currentUmbralRef
				outsideUSDT := math.Abs(diffUSDT) > spikeThreshold
				// The referencial only matters while it is still displayed and
				// while we actually have a value for it. Otherwise a stale or
				// cleaned-up row (Cotizacion=0) would fake a spike every run.
				outsideRef := showReferencial && usdRef.Cotizacion > 0 && math.Abs(diffRef) > spikeThreshold
				isOutside = outsideUSDT || outsideRef
			}

			// A stored messageID must parse to a positive Telegram ID; anything
			// else is treated as absent so the run sends a fresh message and
			// heals the DB, instead of failing every edit forever with id=0.
			restoMid, hasMessageResto := parseMessageID(cfg.MessageID.String)
			usdMid, hasMessageUSD := parseMessageID(cfg.MessageIDUSD.String)

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
				msg, btn := telegram.FormatRestoMessage(summary, baselines)
				return trySend(restoImagePath, msg, true, btn)
			}

			sendUSD := func() (int, error) {
				msg, btn := telegram.FormatUSDMessage(summary, showReferencial, baselines)
				return trySend(usdImagePath, msg, true, btn)
			}

			sendSpikeUSD := func() (int, error) {
				msg, btn := telegram.FormatSpikeUSDMessage(summary, baselines, currentUmbralUSDT, diffUSDT, currentUmbralRef, diffRef, showReferencial)
				return trySend(usdImagePath, msg, false, btn)
			}

			editResto := func(mid int) error {
				msg, btn := telegram.FormatRestoMessage(summary, baselines)
				return editMsg(restoImagePath, mid, msg, btn)
			}

			editUSD := func(mid int) error {
				msg, btn := telegram.FormatUSDMessage(summary, showReferencial, baselines)
				return editMsg(usdImagePath, mid, msg, btn)
			}

			switch {
			case !hasUmbrales:
				ui.Info("Sin umbrales definidos — guardando referencias y omitiendo notificación USD.")
				saveConfigUSD(cfg.MessageIDUSD.String)

				if hasMessageResto {
					ui.Info(fmt.Sprintf("Actualizando mensaje Resto existente (id=%d)...", restoMid))
					if editErr := editResto(restoMid); editErr != nil {
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
						saveMessageIDResto(strconv.Itoa(restoMid))
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
					ui.Info(fmt.Sprintf("Actualizando mensaje Resto existente (id=%d)...", restoMid))
					if editErr := editResto(restoMid); editErr != nil {
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
						saveMessageIDResto(strconv.Itoa(restoMid))
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
					ui.Info(fmt.Sprintf("Actualizando mensaje USD existente (id=%d)...", usdMid))
					if editErr := editUSD(usdMid); editErr != nil {
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
						saveMessageIDUSD(strconv.Itoa(usdMid))
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
	deleted, err := database.DeleteOlderThan(3 * 30 * 24 * time.Hour)
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

// parseMessageID validates a stored Telegram message ID: it must be a
// positive integer. Returns (0,false) for empty/garbage/non-positive values.
func parseMessageID(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
