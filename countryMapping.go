// countryMapping.go
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
)

// CountryCode represents a mapping between country code and name
type CountryCode struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

//go:embed assets/countries.json
var embeddedCountries embed.FS

var countryCodeMap map[string]string
var countryCodeMapInitialized bool = false

// InitCountryCodeMap initializes the country code to full name mapping
func InitCountryCodeMap() error {
	if countryCodeMapInitialized {
		return nil // Already initialized
	}

	// Read the embedded countries.json file
	fileContent, err := embeddedCountries.ReadFile("assets/countries.json")
	if err != nil {
		log.Printf("Error reading embedded countries file: %v", err)
		return fmt.Errorf("error reading embedded countries file: %w", err)
	}

	// Parse the JSON
	var countries []CountryCode
	err = json.Unmarshal(fileContent, &countries)
	if err != nil {
		log.Printf("Error parsing embedded countries file: %v", err)
		return fmt.Errorf("error parsing embedded countries file: %w", err)
	}

	// Create the map
	countryCodeMap = make(map[string]string)
	for _, country := range countries {
		countryCodeMap[country.Code] = country.Name
	}

	// Print debug info
	log.Printf("Loaded %d country codes", len(countryCodeMap))
	if name, ok := countryCodeMap["US"]; ok {
		log.Printf("US maps to: %s", name)
	} else {
		log.Printf("WARNING: US country code not found in mapping")
	}

	countryCodeMapInitialized = true
	return nil
}

// GetCountryName returns the full country name for a given country code
func GetCountryName(code string) string {
	// If the map isn't initialized yet, initialize it
	if !countryCodeMapInitialized {
		if err := InitCountryCodeMap(); err != nil {
			log.Printf("Warning: Failed to initialize country code map: %v", err)
			return code
		}
	}

	// Look up the country name in the map
	if name, ok := countryCodeMap[code]; ok {
		return name
	}

	// If not found, return the original code
	log.Printf("Country code not found: %s", code)
	return code
}
