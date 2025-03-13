package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

func main() {
	// Define command-line flags
	metarOnly := flag.Bool("metar", false, "Show only METAR")
	tafOnly := flag.Bool("taf", false, "Show only TAF")
	noRawFlag := flag.Bool("no-raw", false, "Hide raw data")
	noDecodeFlag := flag.Bool("no-decode", false, "Show only raw data without decoding")
	flagNoColor := flag.Bool("no-color", false, "Disable color output")
	radiusFlag := flag.Float64("radius", 50.0, "Search radius in miles when finding nearest airport (default 50)")
	nearestFlag := flag.Bool("nearest", false, "Find nearest airport to your current location")
	offlineFlag := flag.Bool("offline", false, "Operate in offline mode (only works with stdin data)")
	data := flag.String("data", "", "Decode supplied data only")
	flag.Parse()

	if *flagNoColor {
		color.NoColor = true // disables colorized output globally
	}

	var rawInput string
	if data != nil {
		rawInput = *data
	}

	// First check stdin for piped data
	stationCode, rawInput, stdinHasData, isStdinTAF := readFromStdin(rawInput)

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

	// Handle stdin data based on flags and auto-detection
	if stdinHasData {
		// If offline mode is enabled, get station info from embedded file
		if *offlineFlag {
			// Only attempt to load site info if we don't already have it
			if !siteInfoFetched {
				offlineSiteInfo, err := LoadEmbeddedStationInfo(stationCode)
				if err != nil {
					fmt.Printf("Warning: Could not load offline site info for %s: %v\n", stationCode, err)
				} else {
					siteInfo = offlineSiteInfo
					siteInfoFetched = true
				}
			}
		}

		// Process data according to flags, overriding auto-detection if flags are specified
		if *tafOnly || (isStdinTAF && !*metarOnly) {
			// Process as TAF (either forced with -taf flag or detected as TAF and not forced to METAR)
			processTAF(stationCode, rawInput, true, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched, *offlineFlag)
		} else if *metarOnly || !isStdinTAF {
			// Process as METAR (either forced with -metar flag or detected as METAR)
			processMETAR(stationCode, rawInput, true, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched, *offlineFlag)
		}
	} else {
		// No stdin data, fetch from web based on flags

		// Fetch and display METAR if requested or by default
		if !*tafOnly {
			processMETAR(stationCode, "", false, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched, *offlineFlag)
		}

		// Fetch and display TAF if requested or by default
		if !*metarOnly {
			// Add a line break if we also displayed METAR
			if !*tafOnly {
				fmt.Print("\n----------------------------------\n\n")
			}

			// Fetch and process TAF from the web
			processTAF(stationCode, "", false, *noRawFlag, *noDecodeFlag, siteInfo, siteInfoFetched, *offlineFlag)
		}
	}
}
