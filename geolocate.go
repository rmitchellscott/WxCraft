package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
// Uses ipinfo.io which is free for non-commercial use
func GetLocation() (*Location, error) {
	resp, err := http.Get("https://ipinfo.io/json")
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

	// Parse response from ipinfo.io
	var result struct {
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
		Loc     string `json:"loc"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse lat/lon from "loc" string (e.g., "XX.XXXX,-YY.YYYY")
	latStr, lonStr, found := strings.Cut(result.Loc, ",")
	if !found {
		return nil, fmt.Errorf("invalid loc format: %s", result.Loc)
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid latitude: %s", latStr)
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid longitude: %s", lonStr)
	}

	return &Location{
		Latitude:  lat,
		Longitude: lon,
		City:      result.City,
		Region:    result.Region,
		Country:   GetCountryName(result.Country),
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
