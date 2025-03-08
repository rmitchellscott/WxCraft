package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// GetLocationByZipcode gets location information from a US zipcode
// Uses the public API from zippopotam.us which is free to use
func GetLocationByZipcode(zipcode string) (*Location, error) {
	// Validate zipcode format (basic check)
	if len(zipcode) < 5 {
		return nil, fmt.Errorf("invalid zipcode format: must be at least 5 characters")
	}

	// Build URL with the zipcode
	baseURL := "https://api.zippopotam.us/us/"
	apiURL := baseURL + url.PathEscape(zipcode)

	// Make the request
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("zipcode not found: %s", zipcode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		PostCode string `json:"post code"`
		Country  string `json:"country"`
		Places   []struct {
			PlaceName string `json:"place name"`
			State     string `json:"state"`
			StateAbbr string `json:"state abbreviation"`
			Latitude  string `json:"latitude"`
			Longitude string `json:"longitude"`
		} `json:"places"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Places) == 0 {
		return nil, fmt.Errorf("no location data found for zipcode: %s", zipcode)
	}

	// Parse latitude and longitude from strings to float64
	var lat, lon float64
	place := result.Places[0]

	fmt.Sscanf(place.Latitude, "%f", &lat)
	fmt.Sscanf(place.Longitude, "%f", &lon)

	return &Location{
		Latitude:  lat,
		Longitude: lon,
		City:      place.PlaceName,
		Region:    place.State,
		Country:   result.Country,
	}, nil
}
