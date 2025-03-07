package main

import (
	"bufio"
	"fmt"
	"os"
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
	if len(stationCode) != 4 {
		return "", fmt.Errorf("invalid station code: must be 4 characters")
	}

	return stationCode, nil
}

// promptForStationCode prompts the user for a station code
func promptForStationCode() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ICAO airport code (e.g., KJFK, EGLL): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}

	stationCode := strings.ToUpper(strings.TrimSpace(input))
	if len(stationCode) != 4 {
		return "", fmt.Errorf("invalid station code: must be 4 characters")
	}

	return stationCode, nil
}

// processMETAR fetches, decodes and displays METAR data
func processMETAR(stationCode, rawInput string, stdinHasData, noRaw bool) {
	var metar string
	var err error

	if stdinHasData && rawInput != "" {
		// Use the piped data
		metar = rawInput
	} else {
		// Fetch from the service
		fmt.Printf("Fetching METAR for %s...\n", stationCode)
		metar, err = FetchMETAR(stationCode)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	// Show raw METAR by default, unless --no-raw flag is used
	if !noRaw {
		fmt.Println("\nRaw METAR:")
		fmt.Println(metar)
	}

	fmt.Println("\nDecoded METAR:")
	decoded := DecodeMETAR(metar)
	fmt.Print(FormatMETAR(decoded))
}

// processTAF fetches, decodes and displays TAF data
func processTAF(stationCode string, noRaw bool) {
	fmt.Printf("Fetching TAF for %s...\n", stationCode)

	taf, err := FetchTAF(stationCode)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Show raw TAF by default, unless --no-raw flag is used
	if !noRaw {
		fmt.Println("\nRaw TAF:")
		fmt.Println(taf)
	}

	fmt.Println("\nDecoded TAF:")
	decoded := DecodeTAF(taf)
	fmt.Print(FormatTAF(decoded))
}
