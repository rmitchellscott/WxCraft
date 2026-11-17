package main

import (
	"flag"
	"fmt"

	"github.com/fatih/color"
)

const reportSeparator = "\n----------------------------------\n\n"

func main() {
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
		color.NoColor = true
	}

	lookupSiteInfo := FetchSiteInfo
	if *offlineFlag {
		lookupSiteInfo = LoadEmbeddedStationInfo
	}
	siteInfoByStation := map[string]SiteInfo{}
	siteInfoFor := func(stationCode string) SiteInfo {
		if siteInfo, ok := siteInfoByStation[stationCode]; ok {
			return siteInfo
		}
		siteInfo := SiteInfo{Name: stationCode}
		if !*noDecodeFlag {
			var err error
			siteInfo, err = lookupSiteInfo(stationCode)
			if err != nil {
				fmt.Printf("Warning: Could not get site info for %s: %v\n", stationCode, err)
			}
		}
		siteInfoByStation[stationCode] = siteInfo
		return siteInfo
	}

	input := *data
	if input == "" {
		var err error
		input, err = readPipedStdin()
		if err != nil {
			errorColor.Printf("Error: %v\n", err)
			return
		}
	}

	if input != "" {
		reports := splitReports(input)
		if len(reports) == 0 {
			errorColor.Println("Error: no METAR or TAF found in the input.")
			return
		}
		for i, r := range reports {
			kind := r.Kind
			if *tafOnly {
				kind = tafReport
			} else if *metarOnly {
				kind = metarReport
			}

			if i > 0 {
				fmt.Print(reportSeparator)
			}
			if kind == tafReport {
				processTAF(r.Raw, siteInfoFor(r.Station), *noRawFlag, *noDecodeFlag)
			} else {
				processMETAR(r.Raw, siteInfoFor(r.Station), *noRawFlag, *noDecodeFlag)
			}
		}
		return
	}

	if *offlineFlag {
		errorColor.Println("Error: offline mode needs METAR or TAF data on stdin or in -data.")
		return
	}

	var stationInput string
	switch {
	case *nearestFlag:
		stationInput = "AUTO"
	case flag.NArg() > 0:
		stationInput = flag.Arg(0)
	default:
		var err error
		stationInput, err = promptForStationCode()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	stationCode, err := networkStationResolver.resolve(stationInput, *radiusFlag)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	siteInfo := siteInfoFor(stationCode)

	if !*tafOnly {
		if rawMetar, err := FetchMETAR(stationCode); err != nil {
			errorColor.Printf("Error fetching METAR: %v\n", err)
		} else {
			processMETAR(rawMetar, siteInfo, *noRawFlag, *noDecodeFlag)
		}
	}

	if !*metarOnly {
		if !*tafOnly {
			fmt.Print(reportSeparator)
		}
		if rawTAF, err := FetchTAF(stationCode); err != nil {
			errorColor.Printf("Error fetching TAF: %v\n", err)
		} else {
			processTAF(rawTAF, siteInfo, *noRawFlag, *noDecodeFlag)
		}
	}
}
