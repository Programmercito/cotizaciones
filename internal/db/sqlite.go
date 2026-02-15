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

const dbPath = "/opt/osbo/datausd"

// Cotizacion represents a row in the cotizaciones table
type Cotizacion struct {
	Moneda     string  `json:"moneda"`
	Cotizacion float64 `json:"cotizacion"`
	Datetime   string  `json:"datetime"`
	Exchange   string  `json:"exchange"`
}

// Config represents a row in the config table
type Config struct {
	CurrentDate string
	ChatID      string
	MessageID   sql.NullString
}

// DB wraps the sql.DB connection
type DB struct {
	conn *sql.DB
}

// New opens the SQLite database connection
func New() (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// InsertCotizacion inserts a new cotizacion record
// Uses current local time instead of API time to avoid duplicate key errors
// when the API returns the same cached timestamp across consecutive calls
func (d *DB) InsertCotizacion(bid float64) error {
	datetime := time.Now().Format("2006-01-02 15:04:05")

	_, err := d.conn.Exec(
		"INSERT INTO cotizaciones (moneda, cotizacion, datetime, exchange) VALUES (?, ?, ?, ?)",
		"USDT", bid, datetime, "binancep2p",
	)
	if err != nil {
		return fmt.Errorf("error inserting cotizacion: %w", err)
	}

	return nil
}

// GetAllCotizaciones retrieves all records from the cotizaciones table
func (d *DB) GetAllCotizaciones() ([]Cotizacion, error) {
	rows, err := d.conn.Query("SELECT moneda, cotizacion, datetime, exchange FROM cotizaciones ORDER BY datetime ASC")
	if err != nil {
		return nil, fmt.Errorf("error querying cotizaciones: %w", err)
	}
	defer rows.Close()

	var cotizaciones []Cotizacion
	for rows.Next() {
		var c Cotizacion
		if err := rows.Scan(&c.Moneda, &c.Cotizacion, &c.Datetime, &c.Exchange); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		cotizaciones = append(cotizaciones, c)
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

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("error writing JSON file: %w", err)
	}

	return nil
}

// GetConfig retrieves the config record
func (d *DB) GetConfig() (*Config, error) {
	var cfg Config
	err := d.conn.QueryRow("SELECT currentdate, chatid, messageid FROM config LIMIT 1").
		Scan(&cfg.CurrentDate, &cfg.ChatID, &cfg.MessageID)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}
	return &cfg, nil
}

// UpdateConfig updates the currentdate and messageid in the config table
func (d *DB) UpdateConfig(currentDate string, messageID string) error {
	_, err := d.conn.Exec(
		"UPDATE config SET currentdate = ?, messageid = ?",
		currentDate, messageID,
	)
	if err != nil {
		return fmt.Errorf("error updating config: %w", err)
	}
	return nil
}
