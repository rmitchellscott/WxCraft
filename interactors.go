package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// readFromStdin reads data from stdin if available
func readFromStdin() (string, string, bool, bool) {
	// Check if input is being piped in (stdin)
	info, err := os.Stdin.Stat()
	stdinHasData := (err == nil && info.Mode()&os.ModeCharDevice == 0)

	if !stdinHasData {
		return "", "", false, false
	}

	// Read from stdin if data is piped in
	scanner := bufio.NewScanner(os.Stdin)
  
	// First, read the complete input which might span multiple lines
	var inputBuilder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		inputBuilder.WriteString(line)
		inputBuilder.WriteString("\n") // Preserve line breaks
	}

  rawInput := strings.TrimSpace(inputBuilder.String())

	// If we couldn't read any data, return
	if rawInput == "" {
		return "", "", false, false
	}

	// Try to extract station code from the first line of raw input
	lines := strings.Split(rawInput, "\n")
	firstLine := lines[0]
	parts := strings.Fields(firstLine)

  	if len(parts) > 0 {
		// Determine if input is a TAF or METAR
		// Look for TAF-specific keywords and patterns
		isTAF := strings.HasPrefix(strings.TrimSpace(firstLine), "TAF") ||
			strings.Contains(rawInput, "TEMPO") ||
			strings.Contains(rawInput, "BECMG") ||
			strings.Contains(rawInput, "PROB") ||
			// The following regex matches a typical TAF valid period format (e.g., 1106/1212)
			regexp.MustCompile(`\d{4}/\d{4}`).MatchString(rawInput)
		// If the first token is "TAF", use the second token as the station code
		stationCode := parts[0]
		if stationCode == "TAF" && len(parts) > 1 {
			stationCode = parts[1]
		}

		return stationCode, rawInput, true, isTAF
	}

	return "", "", false, false
}

// getStationCodeFromArgs gets station code from command-line args
func getStationCodeFromArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("no station code provided")
	}

	stationCode := strings.ToUpper(strings.TrimSpace(args[0]))

	// Check for AUTO keyword
	if stationCode == "AUTO" {
		return "AUTO", nil // Return AUTO instead of handling it here
	}

	// Check for zipcode format (5-digit or ZIP+4)
	zipRegex := regexp.MustCompile(`^\d{5}(-\d{4})?$`)
	if zipRegex.MatchString(stationCode) {
		return stationCode, nil // Return zipcode instead of handling it here
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

	// Return the raw input instead of processing it here
	return stationCode, nil
}
