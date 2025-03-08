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
	flag.Parse()

	if *flagNoColor {
		color.NoColor = true // disables colorized output globally
	}

	// First check stdin for piped data
	stationCode, rawInput, stdinHasData := readFromStdin()

	// If no stdin data, get station code from args or prompt
	if !stdinHasData {
		var err error

		// Try command line args first
		remainingArgs := flag.Args()
		if len(remainingArgs) > 0 {
			stationCode, err = getStationCodeFromArgs(remainingArgs)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		} else {
			// Prompt the user
			stationCode, err = promptForStationCode()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		}
	}

	// Pre-fetch site information if we're going to decode the data
	// and both METAR and TAF are requested
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
