package main

import (
	"fmt"
	"os"
	"time"

	"cotizaciones/internal/api"
	"cotizaciones/internal/db"
	"cotizaciones/internal/git"
	"cotizaciones/internal/social"
	"cotizaciones/internal/ui"

	"github.com/joho/godotenv"
)

const (
	jsonOutputPath = "/opt/codes/cotizaciones_ng/docs/data.json"
	ngRepoPath     = "/opt/codes/cotizaciones_ng"
)

func main() {
	ui.Banner()

	if err := godotenv.Load(); err != nil {
		ui.Warn(".env no encontrado, usando variables de entorno del sistema")
	}

	ui.StepStart(1, 6, "🌐", "Consultando API de CriptoYa...")
	data, err := api.FetchCotizacion()
	if err != nil {
		exitWithError("Error consultando API: %v", err)
	}
	ui.Success("Respuesta recibida correctamente")
	ui.Prices(data.Bid, data.TotalAsk)

	ui.StepStart(2, 6, "🗄️", "Conectando a base de datos SQLite...")
	database, err := db.New()
	if err != nil {
		exitWithError("Error abriendo base de datos: %v", err)
	}
	defer database.Close()
	ui.Success("Conexión establecida")

	ui.StepStart(3, 6, "💾", "Guardando cotización en base de datos...")
	if err := database.InsertCotizacion(data.Bid, data.TotalAsk); err != nil {
		exitWithError("Error guardando cotización: %v", err)
	}
	ui.Success("Cotización guardada → moneda=USDT exchange=binancep2p")

	summary, err := database.GetLatestSummary()
	if err != nil {
		exitWithError("Error obteniendo resumen: %v", err)
	}
	baselines, err := database.GetUSDBaselines()
	if err != nil {
		exitWithError("Error obteniendo referencias del día anterior: %v", err)
	}
	if _, err := social.GenerateSocialData(summary, baselines); err != nil {
		exitWithError("Error generando JSON para redes: %v", err)
	}
	ui.Success("JSON para redes generado → /opt/osbo/cotiza/social.json")

	ui.StepStart(4, 6, "🔄", "Actualizando repositorio frontend...")
	if err := git.ForcePull(ngRepoPath); err != nil {
		exitWithError("Error en git pull: %v", err)
	}
	ui.Success(fmt.Sprintf("Repositorio actualizado → %s", ngRepoPath))

	ui.StepStart(5, 6, "📄", "Exportando cotizaciones a JSON...")
	if err := database.ExportCotizacionesToJSON(jsonOutputPath); err != nil {
		exitWithError("Error exportando JSON: %v", err)
	}
	ui.Success(fmt.Sprintf("Archivo generado → %s", jsonOutputPath))

	ui.StepStart(6, 6, "🚀", "Publicando JSON y limpiando registros...")
	if err := git.CommitAndPush(ngRepoPath, "data upload"); err != nil {
		exitWithError("Error en git push: %v", err)
	}
	deleted, err := database.DeleteOlderThan(3 * 30 * 24 * time.Hour)
	if err != nil {
		exitWithError("Error limpiando registros: %v", err)
	}
	ui.Success(fmt.Sprintf("Cambios subidos; registros antiguos eliminados: %d", deleted))
	ui.Done()
}

func exitWithError(format string, args ...any) {
	ui.Fatal(fmt.Sprintf(format, args...))
	os.Exit(1)
}
