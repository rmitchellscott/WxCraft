package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Location represents geographic coordinates of the user
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	Country   string  `json:"country"`
}

// GetLocation uses a free IP geolocation service to get location information
// Uses ip-api.com which is free for non-commercial use
func GetLocation() (*Location, error) {
	resp, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response from ip-api.com
	var result struct {
		Status      string  `json:"status"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		City        string  `json:"city"`
		RegionName  string  `json:"regionName"`
		CountryName string  `json:"country"`
		Message     string  `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("geolocation failed: %s", result.Message)
	}

	return &Location{
		Latitude:  result.Lat,
		Longitude: result.Lon,
		City:      result.City,
		Region:    result.RegionName,
		Country:   result.CountryName,
	}, nil
}
