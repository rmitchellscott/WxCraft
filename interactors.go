package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// readFromStdin reads data from stdin if available
func readFromStdin() (string, string, bool) {
	// Check if input is being piped in (stdin)
	info, err := os.Stdin.Stat()
	stdinHasData := (err == nil && info.Mode()&os.ModeCharDevice == 0)

	if !stdinHasData {
		return "", "", false
	}

	// Read from stdin if data is piped in
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		rawInput := scanner.Text()

		// Try to extract station code from the raw input
		parts := strings.Fields(rawInput)
		if len(parts) > 0 {
			return parts[0], rawInput, true
		}
	}

	return "", "", false
}

// getStationCodeFromArgs gets station code from command-line args
func getStationCodeFromArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("no station code provided")
	}

	stationCode := strings.ToUpper(strings.TrimSpace(args[0]))

	// Check for AUTO keyword
	if stationCode == "AUTO" {
		return handleAutoLocation(50.0) // Default radius of 50 miles
	}

	// Check for zipcode format (5-digit or ZIP+4)
	zipRegex := regexp.MustCompile(`^\d{5}(-\d{4})?$`)
	if zipRegex.MatchString(stationCode) {
		return handleZipcodeLocation(stationCode, 50.0) // Default radius of 50 miles
	}

	// Check for ICAO format
	if len(stationCode) != 4 {
		return "", fmt.Errorf("invalid station code: must be 4 characters")
	}

	return stationCode, nil
}

// promptForStationCode prompts the user for a station code
func promptForStationCode() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ICAO airport code (e.g., KJFK, EGLL), US zipcode, or 'AUTO' for nearest airport: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}

	stationCode := strings.ToUpper(strings.TrimSpace(input))

	// Check for AUTO keyword
	if stationCode == "AUTO" {
		return handleAutoLocation(50.0) // Default radius of 50 miles
	}

	// Check for zipcode format (5-digit or ZIP+4)
	zipRegex := regexp.MustCompile(`^\d{5}(-\d{4})?$`)
	if zipRegex.MatchString(stationCode) {
		return handleZipcodeLocation(stationCode, 50.0) // Default radius of 50 miles
	}

	// Check for ICAO format
	if len(stationCode) != 4 {
		return "", fmt.Errorf("invalid station code: must be 4 characters")
	}

	return stationCode, nil
}

// handleAutoLocation determines the nearest airport based on IP geolocation
func handleAutoLocation(radiusMiles float64) (string, error) {
	fmt.Println("Finding nearest airport to your location...")
	location, err := GetLocation()
	if err != nil {
		return "", fmt.Errorf("failed to get your location: %v", err)
	}

	fmt.Printf("Your location: %s, %s (%.4f, %.4f)\n",
		location.City, location.Country,
		location.Latitude, location.Longitude)

	// Get the nearest airport ICAO code
	fmt.Printf("Searching for airports within %.1f miles...\n", radiusMiles)
	icaoCode, distance, err := GetNearestAirportICAO(
		location.Latitude,
		location.Longitude,
		radiusMiles,
	)

	if err != nil {
		return "", err
	}

	fmt.Printf("Nearest airport: %s (%.1f miles away)\n", icaoCode, distance)
	return icaoCode, nil
}

// handleZipcodeLocation determines the nearest airport based on zipcode
func handleZipcodeLocation(zipcode string, radiusMiles float64) (string, error) {
	fmt.Printf("Looking up location for zipcode %s...\n", zipcode)
	location, err := GetLocationByZipcode(zipcode)
	if err != nil {
		return "", fmt.Errorf("failed to get location for zipcode: %v", err)
	}

	fmt.Printf("Zipcode location: %s, %s, %s (%.4f, %.4f)\n",
		location.City, location.Region, location.Country,
		location.Latitude, location.Longitude)

	// Get the nearest airport ICAO code
	fmt.Printf("Searching for airports within %.1f miles...\n", radiusMiles)
	icaoCode, distance, err := GetNearestAirportICAO(
		location.Latitude,
		location.Longitude,
		radiusMiles,
	)

	if err != nil {
		return "", err
	}

	fmt.Printf("Nearest airport: %s (%.1f miles away)\n", icaoCode, distance)
	return icaoCode, nil
}
