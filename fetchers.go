package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "WxCraft"

var httpClient = &http.Client{Timeout: 10 * time.Second}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return httpClient.Do(req)
}

// fetchData fetches data from a URL for a given station code
func fetchData(urlTemplate string, stationCode string, dataType string) (string, error) {
	resp, err := httpGet(fmt.Sprintf(urlTemplate, url.QueryEscape(stationCode)))
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
	defaultSiteInfo := SiteInfo{Name: stationCode}

	text, err := fetchData("https://aviationweather.gov/api/data/stationinfo?ids=%s&format=json", stationCode, "site info")
	if err != nil {
		return defaultSiteInfo, err
	}

	var stations []StationData
	if err := json.Unmarshal([]byte(text), &stations); err != nil {
		return defaultSiteInfo, fmt.Errorf("error parsing site info: %w", err)
	}
	if len(stations) == 0 || stations[0].Site == "" {
		return defaultSiteInfo, fmt.Errorf("could not extract site name from response")
	}

	station := stations[0]
	country := station.Country
	if len(country) == 2 {
		country = GetCountryName(country)
	}
	return SiteInfo{
		Name:      strings.TrimSpace(station.Site),
		State:     station.State,
		Country:   country,
		Elevation: station.Elev,
	}, nil
}
