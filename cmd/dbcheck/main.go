package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "/opt/osbo/datausd")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT moneda, cotizacion, purchase, datetime, exchange, moneda_dest FROM cotizaciones WHERE moneda LIKE '%eur%' OR moneda LIKE '%Euro%' ORDER BY moneda, datetime DESC LIMIT 5")
	if err != nil {
		fmt.Println("Query error:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m, dt, ex string
		var c, p float64
		var md sql.NullString
		rows.Scan(&m, &c, &p, &dt, &ex, &md)
		fmt.Printf("%-6s | cot=%-12.5f | pur=%-12.5f | dt=%-22s | ex=%-10s | dest=%s\n", m, c, p, dt, ex, md.String)
	}
}
