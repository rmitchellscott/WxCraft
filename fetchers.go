package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
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
	return fetchData("https://aviationweather.gov/cgi-bin/data/metar.php?ids=%s", stationCode, "METAR")
}

// FetchTAF fetches the raw TAF for a given station code
func FetchTAF(stationCode string) (string, error) {
	return fetchData("https://aviationweather.gov/cgi-bin/data/taf.php?ids=%s", stationCode, "TAF")
}
