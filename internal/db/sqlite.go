package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	dbPath   = "/opt/osbo/datausd"
	moneda   = "USDT"
	exchange = "binancep2p"
	timeFmt  = "2006-01-02 15:04:05"

	// TimeFmt es el formato de almacenamiento en DB (exportado para uso en otros paquetes)
	TimeFmt = timeFmt
	// DisplayTimeFmt es el formato legible para mostrar fecha y hora con segundos.
	DisplayTimeFmt = "02/01/2006 · 15:04:05"
	// DisplayDateFmt es el formato legible para mostrar solo la fecha.
	DisplayDateFmt = "02/01/2006"
)

// Cotizacion represents a row in the cotizaciones table
type Cotizacion struct {
	Moneda     string  `json:"moneda"`
	Cotizacion float64 `json:"cotizacion"`
	Purchase   float64 `json:"purchase"`
	Datetime   string  `json:"datetime"`
	Exchange   string  `json:"exchange"`
	MonedaDest string  `json:"moneda_dest,omitempty"` // para futuras conversiones
}

// Config represents a row in the config table
type Config struct {
	CurrentDate       string
	ChatID            string
	MessageID         sql.NullString
	Umbral            sql.NullFloat64 // referencia USDT
	UmbralReferencial sql.NullFloat64 // referencia USD Referencial
}

// DB wraps the sql.DB connection
type DB struct {
	conn *sql.DB
}

// New opens the SQLite database connection and applies performance pragmas
func New() (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("error setting WAL mode: %w", err)
	}

	// Ensure 'purchase' column exists (migration)
	_, _ = conn.Exec("ALTER TABLE cotizaciones ADD COLUMN purchase REAL DEFAULT 0")
	// Ensure 'umbral_referencial' column exists (migration)
	_, _ = conn.Exec("ALTER TABLE config ADD COLUMN umbral_referencial REAL")

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// InsertCotizacion inserts a new cotizacion record.
// Uses current local time to avoid duplicate key errors when the API
// returns the same cached timestamp across consecutive calls.
func (d *DB) InsertCotizacion(bid, purchase float64) error {
	datetime := time.Now().Format(timeFmt)

	_, err := d.conn.Exec(
		"INSERT INTO cotizaciones (moneda, cotizacion, purchase, datetime, exchange) VALUES (?, ?, ?, ?, ?)",
		moneda, bid, purchase, datetime, exchange,
	)
	if err != nil {
		return fmt.Errorf("error inserting cotizacion: %w", err)
	}

	return nil
}

// GetAllCotizaciones retrieves all records from the cotizaciones table
func (d *DB) GetAllCotizaciones() ([]Cotizacion, error) {
	rows, err := d.conn.Query(
		"SELECT moneda, cotizacion, purchase, datetime, exchange, moneda_dest FROM cotizaciones ORDER BY datetime ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("error querying cotizaciones: %w", err)
	}
	defer rows.Close()

	var cotizaciones []Cotizacion
	for rows.Next() {
		var c Cotizacion
		if err := rows.Scan(&c.Moneda, &c.Cotizacion, &c.Purchase, &c.Datetime, &c.Exchange, &c.MonedaDest); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		cotizaciones = append(cotizaciones, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return cotizaciones, nil
}

// ExportCotizacionesToJSON exports all cotizaciones to a JSON file
func (d *DB) ExportCotizacionesToJSON(outputPath string) error {
	cotizaciones, err := d.GetAllCotizaciones()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cotizaciones, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("error writing JSON file: %w", err)
	}

	return nil
}

// GetConfig retrieves the single config record
func (d *DB) GetConfig() (*Config, error) {
	var cfg Config
	err := d.conn.QueryRow("SELECT currentdate, chatid, messageid, umbral, umbral_referencial FROM config LIMIT 1").
		Scan(&cfg.CurrentDate, &cfg.ChatID, &cfg.MessageID, &cfg.Umbral, &cfg.UmbralReferencial)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}
	return &cfg, nil
}

// UpdateConfig updates currentdate, messageid, umbral (USDT) y umbral_referencial (USD Ref)
func (d *DB) UpdateConfig(currentDate, messageID string, umbralUSDT, umbralRef float64) error {
	var mID any = messageID
	if messageID == "" {
		mID = nil
	}
	res, err := d.conn.Exec(
		"UPDATE config SET currentdate = ?, messageid = ?, umbral = ?, umbral_referencial = ? WHERE rowid = (SELECT rowid FROM config LIMIT 1)",
		currentDate, mID, umbralUSDT, umbralRef,
	)
	if err != nil {
		return fmt.Errorf("error updating config: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("config: ninguna fila actualizada (tabla vacía?)")
	}
	return nil
}

// DeleteOlderThan deletes cotizaciones older than the given duration and returns the count deleted.
func (d *DB) DeleteOlderThan(d1 time.Duration) (int64, error) {
	cutoff := time.Now().Add(-d1).Format(timeFmt)
	result, err := d.conn.Exec("DELETE FROM cotizaciones WHERE datetime < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("error deleting old cotizaciones: %w", err)
	}
	return result.RowsAffected()
}

// GetLatestByMoneda returns the most recent cotizacion for a specific moneda
func (d *DB) GetLatestByMoneda(name string) (Cotizacion, error) {
	var c Cotizacion
	err := d.conn.QueryRow(
		"SELECT moneda, cotizacion, purchase, datetime, exchange FROM cotizaciones WHERE moneda = ? ORDER BY datetime DESC LIMIT 1",
		name,
	).Scan(&c.Moneda, &c.Cotizacion, &c.Purchase, &c.Datetime, &c.Exchange)

	if err != nil {
		return Cotizacion{}, err
	}
	return c, nil
}

// GetLatestSummary returns a map of the latest quotes for the three main types
func (d *DB) GetLatestSummary() (map[string]Cotizacion, error) {
	summary := make(map[string]Cotizacion)

	monedas := []string{"USDT", "usd oficial", "usd referencial", "euro", "oro", "plata", "ufv"}
	for _, m := range monedas {
		c, err := d.GetLatestByMoneda(m)
		if err == nil {
			summary[m] = c
		} else if err != sql.ErrNoRows {
			return nil, fmt.Errorf("error fetching %s: %w", m, err)
		}
	}

	return summary, nil
}
