package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// fetchData fetches data from a URL for a given station code
func fetchData(urlTemplate string, stationCode string, dataType string) (string, error) {
	url := fmt.Sprintf(urlTemplate, stationCode)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching %s: %w", dataType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	data := strings.TrimSpace(string(body))
	if data == "" {
		return "", fmt.Errorf("no %s data found for station %s", dataType, stationCode)
	}

	return data, nil
}

// FetchMETAR fetches the raw METAR for a given station code
func FetchMETAR(stationCode string) (string, error) {
	return fetchData("https://aviationweather.gov/api/data/metar?ids=%s", stationCode, "METAR")
}

// FetchTAF fetches the raw TAF for a given station code
func FetchTAF(stationCode string) (string, error) {
	return fetchData("https://aviationweather.gov/api/data/taf?ids=%s", stationCode, "TAF")
}

// FetchSiteInfo fetches site information for a station from the Aviation Weather API
func FetchSiteInfo(stationCode string) (SiteInfo, error) {
	// Default site info in case of error
	defaultSiteInfo := SiteInfo{
		Name:    stationCode,
		State:   "",
		Country: "",
	}

	// API endpoint for station information
	url := fmt.Sprintf("https://aviationweather.gov/api/data/stationinfo?ids=%s", stationCode)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(url)
	if err != nil {
		return defaultSiteInfo, fmt.Errorf("error fetching site data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return defaultSiteInfo, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return defaultSiteInfo, fmt.Errorf("error reading response: %w", err)
	}

	// Parse the text response using regex
	text := string(body)

	// Extract site information using regular expressions
	var siteName, state, country string

	// Match Site: line
	siteRegex := regexp.MustCompile(`Site:\s+(.+)`)
	siteMatches := siteRegex.FindStringSubmatch(text)
	if len(siteMatches) > 1 {
		siteName = strings.TrimSpace(siteMatches[1])
	}

	// Match State: line
	stateRegex := regexp.MustCompile(`State:\s+(.+)`)
	stateMatches := stateRegex.FindStringSubmatch(text)
	if len(stateMatches) > 1 {
		state = strings.TrimSpace(stateMatches[1])
	}

	// Match Country: line
	countryRegex := regexp.MustCompile(`Country:\s+(.+)`)
	countryMatches := countryRegex.FindStringSubmatch(text)
	if len(countryMatches) > 1 {
		country = strings.TrimSpace(countryMatches[1])

		// If we have a country code, convert it to the full name
		if len(country) == 2 {
			country = GetCountryName(country)
		}
	}

	// If we couldn't extract site name, return an error (state/country optional)
	if siteName == "" {
		return defaultSiteInfo, fmt.Errorf("could not extract site name from response")
	}

	return SiteInfo{
		Name:    siteName,
		State:   state,
		Country: country,
	}, nil
}
