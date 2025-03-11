package main

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed assets/stations.json
var embeddedFiles embed.FS

// Station represents a weather station from the embedded database
type StationData struct {
	Country  string  `json:"country"`
	Elev     int     `json:"elev"`
	FAAId    string  `json:"faaId"`
	IATAId   string  `json:"iataId"`
	ICAOId   string  `json:"icaoId"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Priority int     `json:"priority"`
	Site     string  `json:"site"`
	State    string  `json:"state"`
	WMOId    string  `json:"wmoId"`
}

// LoadEmbeddedStationInfo loads station information from the embedded stations.json file
func LoadEmbeddedStationInfo(stationCode string) (SiteInfo, error) {
	// Default site info in case of error
	defaultSiteInfo := SiteInfo{
		Name:    stationCode,
		State:   "",
		Country: "",
	}

	// Read the embedded stations.json file
	fileContent, err := embeddedFiles.ReadFile("assets/stations.json")
	if err != nil {
		return defaultSiteInfo, fmt.Errorf("error reading embedded stations file: %w", err)
	}

	// Parse the JSON
	var stations []StationData
	err = json.Unmarshal(fileContent, &stations)
	if err != nil {
		return defaultSiteInfo, fmt.Errorf("error parsing embedded stations file: %w", err)
	}

	// Look up the station by its ICAO code
	for _, station := range stations {
		if station.ICAOId == stationCode {
			return SiteInfo{
				Name:    station.Site,
				State:   station.State,
				Country: station.Country,
			}, nil
		}
	}

	return defaultSiteInfo, fmt.Errorf("station %s not found in embedded database", stationCode)
}
