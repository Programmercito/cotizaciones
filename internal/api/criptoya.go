package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiURL = "https://criptoya.com/api/binancep2p/USDT/BOB"

// CriptoYaResponse represents the API response from criptoya.com
type CriptoYaResponse struct {
	Ask      float64 `json:"ask"`
	TotalAsk float64 `json:"totalAsk"`
	Bid      float64 `json:"bid"`
	TotalBid float64 `json:"totalBid"`
	Time     int64   `json:"time"`
}

// FetchCotizacion fetches the current USDT/BOB quote from CriptoYa API
func FetchCotizacion() (*CriptoYaResponse, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var data CriptoYaResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return &data, nil
}
