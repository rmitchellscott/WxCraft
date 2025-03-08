package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// isWeatherCode checks if a string contains any weather codes
func isWeatherCode(s string) bool {
	// Don't match cloud patterns as weather
	if strings.HasPrefix(s, "SKC") ||
		strings.HasPrefix(s, "CLR") ||
		strings.HasPrefix(s, "FEW") ||
		strings.HasPrefix(s, "SCT") ||
		strings.HasPrefix(s, "BKN") ||
		strings.HasPrefix(s, "OVC") {
		return false
	}

	for code := range weatherCodes {
		if strings.Contains(s, code) {
			return true
		}
	}
	return false
}
func main() {
	// Define command-line flags
	metarOnly := flag.Bool("metar", false, "Show only METAR")
	tafOnly := flag.Bool("taf", false, "Show only TAF")
	noRawFlag := flag.Bool("no-raw", false, "Hide raw data")
	noDecodeFlag := flag.Bool("no-decode", false, "Show only raw data without decoding")
	flagNoColor := flag.Bool("no-color", false, "Disable color output")
	radiusFlag := flag.Float64("radius", 50.0, "Search radius in miles when finding nearest airport (default 50)")
	nearestFlag := flag.Bool("nearest", false, "Find nearest airport to your current location")
	flag.Parse()

	if *flagNoColor {
		color.NoColor = true // disables colorized output globally
	}

	// First check stdin for piped data
	stationCode, rawInput, stdinHasData := readFromStdin()

	// If no stdin data, get station code from various sources
	if !stdinHasData {
		var err error

		// Check if -nearest flag is used
		if *nearestFlag {
			stationCode, err = ProcessAutoCommand(*radiusFlag)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		} else {
			// Try command line args first
			remainingArgs := flag.Args()
			if len(remainingArgs) > 0 {
				input := strings.ToUpper(strings.TrimSpace(remainingArgs[0]))

				// Check for special cases before calling the standard function
				if input == "AUTO" {
					stationCode, err = ProcessAutoCommand(*radiusFlag)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						return
					}
				} else if zipRegex.MatchString(input) {
					stationCode, err = ProcessZipcode(input, *radiusFlag)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						return
					}
				} else {
					// Use existing function for regular ICAO codes
					stationCode, err = getStationCodeFromArgs(remainingArgs)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						return
					}
				}
			} else {
				// Prompt the user
				stationCode, err = promptForStationCode()
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}

				// Check for special cases after getting user input
				if stationCode == "AUTO" {
					stationCode, err = ProcessAutoCommand(*radiusFlag)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						return
					}
				} else if zipRegex.MatchString(stationCode) {
					stationCode, err = ProcessZipcode(stationCode, *radiusFlag)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						return
					}
				}
			}
		}
	}

	// Rest of your code unchanged...
	var siteInfo SiteInfo
	var siteInfoFetched bool

	if !*noDecodeFlag {
		fetchedSiteInfo, err := FetchSiteInfo(stationCode)
		if err != nil {
			fmt.Printf("Warning: Could not fetch site info for %s: %v\n", stationCode, err)
		} else {
			siteInfo = fetchedSiteInfo
			siteInfoFetched = true
		}
	}

	// Fetch and display METAR if requested or by default
	if !*tafOnly {
		processMETAR(stationCode, rawInput, stdinHasData, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched)
	}

	// Fetch and display TAF if requested or by default
	if !*metarOnly && !stdinHasData {
		// Add a line break if we also displayed METAR
		if !*tafOnly {
			fmt.Println("\n----------------------------------\n")
		}

		processTAF(stationCode, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched)
	}
}
