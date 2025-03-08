package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// processRemarks processes the remarks section of a METAR
func processRemarks(remarkParts []string) []Remark {
	remarks := []Remark{}

	// Common remark codes and their descriptions
	remarkCodes := map[string]string{
		"AO1":    "automated station without precipitation sensor",
		"AO2":    "automated station with precipitation sensor",
		"SLP":    "sea level pressure",
		"RMK":    "remarks indicator",
		"PRESRR": "pressure rising rapidly",
		"PRESFR": "pressure falling rapidly",
		"NOSIG":  "no significant changes expected",
		"TEMPO":  "temporary",
		"BECMG":  "becoming",
		"VIRGA":  "precipitation not reaching ground",
		"FROPA":  "frontal passage",
	}

	// Process individual remarks or groups of related remarks
	i := 0
	for i < len(remarkParts) {
		part := remarkParts[i]

		// Handle peak wind
		if strings.HasPrefix(part, "PK") && i+2 < len(remarkParts) {
			windRegex := regexp.MustCompile(`^PK\s+WND\s+(\d{3})(\d{2,3})/(\d{2})(\d{2})$`)
			if windRegex.MatchString(strings.Join(remarkParts[i:i+3], " ")) {
				matches := windRegex.FindStringSubmatch(strings.Join(remarkParts[i:i+3], " "))
				dir := matches[1]
				speed := matches[2]
				hour := matches[3]
				minute := matches[4]

				remarks = append(remarks, Remark{
					Raw:         strings.Join(remarkParts[i:i+3], " "),
					Description: fmt.Sprintf("peak wind %s° at %s knots at %s:%s", dir, speed, hour, minute),
				})
				i += 3
				continue
			}
		}

		// Handle sea level pressure
		if strings.HasPrefix(part, "SLP") {
			slpValue := part[3:] // This gets "093" from "SLP093"
			slp, err := strconv.Atoi(slpValue)
			if err == nil {
				// SLP is given in tenths of millibars with an implied leading 10 or 9
				var prefix float64 = 1000.0
				if slp >= 500 {
					prefix = 900.0
				}
				slpHpa := prefix + float64(slp)/10
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("sea level pressure %.1f hPa", slpHpa),
				})
			} else {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: "sea level pressure (invalid format)",
				})
			}
			i++
			continue
		}

		// Handle Temperature/Dew Point in tenths of degrees
		tempDetailedRegex := regexp.MustCompile(`^T(\d)(\d{3})(\d)(\d{3})$`)
		if tempDetailedRegex.MatchString(part) {
			matches := tempDetailedRegex.FindStringSubmatch(part)
			tempSign := matches[1]
			tempVal := matches[2]
			dewSign := matches[3]
			dewVal := matches[4]

			temp, _ := strconv.Atoi(tempVal)
			dew, _ := strconv.Atoi(dewVal)

			// Convert to tenths of degrees
			tempF := float64(temp) / 10.0
			dewF := float64(dew) / 10.0

			if tempSign == "1" {
				tempF = -tempF
			}
			if dewSign == "1" {
				dewF = -dewF
			}

			remarks = append(remarks, Remark{
				Raw:         part,
				Description: fmt.Sprintf("temperature %.1f°C, dew point %.1f°C", tempF, dewF),
			})
			i++
			continue
		}

		// Handle 6-hour maximum temperature (format: 1sTTT)
		if len(part) == 5 && part[0] == '1' {
			sign := part[1]
			tempStr := part[2:]
			temp, err := strconv.Atoi(tempStr)
			if err == nil {
				tempValue := float64(temp) / 10.0 // Convert to degrees
				if sign == '1' {
					tempValue = -tempValue // Apply negative sign if needed
				}
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("6-hour maximum temperature %.1f°C", tempValue),
				})
				i++
				continue
			}
		}

		// Handle 6-hour minimum temperature (format: 2sTTT)
		if len(part) == 5 && part[0] == '2' {
			sign := part[1]
			tempStr := part[2:]
			temp, err := strconv.Atoi(tempStr)
			if err == nil {
				tempValue := float64(temp) / 10.0 // Convert to degrees
				if sign == '1' {
					tempValue = -tempValue // Apply negative sign if needed
				}
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("6-hour minimum temperature %.1f°C", tempValue),
				})
				i++
				continue
			}
		}

		// Handle 3-hour pressure change (format: 3PPPP)
		if len(part) == 5 && part[0] == '3' {
			pressStr := part[1:]
			press, err := strconv.Atoi(pressStr)
			if err == nil {
				hpa := float64(press) / 10.0 // Convert to hPa
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("3-hour pressure change: %.1f hPa", hpa),
				})
				i++
				continue
			}
		}

		// Handle pressure tendency (format: 5appp)
		if len(part) == 5 && part[0] == '5' {
			tendencyCode := part[1]
			changeStr := part[2:]
			change, err := strconv.Atoi(changeStr)
			if err == nil {
				changeValue := float64(change) / 10.0 // Convert to hPa

				tendencyDesc := "unknown"
				switch tendencyCode {
				case '0':
					tendencyDesc = "increasing, then decreasing"
				case '1':
					tendencyDesc = "increasing, then steady"
				case '2':
					tendencyDesc = "increasing steadily"
				case '3':
					tendencyDesc = "increasing, then increasing more rapidly"
				case '4':
					tendencyDesc = "steady"
				case '5':
					tendencyDesc = "decreasing, then increasing"
				case '6':
					tendencyDesc = "decreasing, then steady"
				case '7':
					tendencyDesc = "decreasing steadily"
				case '8':
					tendencyDesc = "decreasing, then decreasing more rapidly"
				}

				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("pressure tendency: %s, %.1f hPa change", tendencyDesc, changeValue),
				})
				i++
				continue
			}
		}

		// Handle precipitation amounts
		if precRegex := regexp.MustCompile(`^P(\d{4})$`); precRegex.MatchString(part) {
			matches := precRegex.FindStringSubmatch(part)
			precip, _ := strconv.Atoi(matches[1])
			inches := float64(precip) / 100.0

			remarks = append(remarks, Remark{
				Raw:         part,
				Description: fmt.Sprintf("precipitation of %.2f inches in the last hour", inches),
			})
			i++
			continue
		}

		// Handle 24-hour precipitation (format: 7RRRR)
		if len(part) == 5 && part[0] == '7' {
			precipStr := part[1:]
			precip, err := strconv.Atoi(precipStr)
			if err == nil {
				inches := float64(precip) / 100.0 // Convert to inches
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("24-hour precipitation: %.2f inches", inches),
				})
				i++
				continue
			}
		}

		// Handle snow depth on ground (format: 4/sss)
		if strings.HasPrefix(part, "4/") && len(part) == 5 {
			snowStr := part[2:]
			snow, err := strconv.Atoi(snowStr)
			if err == nil {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("snow depth: %d inches", snow),
				})
				i++
				continue
			}
		}

		// Handle ice accretion (format: IhVV)
		if len(part) == 5 && part[0] == 'I' && part[1] >= '1' && part[1] <= '3' {
			hourDigit := part[1]
			accretionStr := part[2:]
			accretion, err := strconv.Atoi(accretionStr)
			if err == nil {
				hours := map[byte]string{
					'1': "1-hour",
					'2': "3-hour",
					'3': "6-hour",
				}

				timeframe := hours[hourDigit]
				inches := float64(accretion) / 100.0 // Convert to inches

				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("%s ice accretion: %.2f inches", timeframe, inches),
				})
				i++
				continue
			}
		}

		// Handle recent weather (format: REww)
		if strings.HasPrefix(part, "RE") && len(part) >= 4 {
			wxType := part[2:]
			weatherMap := map[string]string{
				"RA": "rain",
				"SN": "snow",
				"GR": "hail",
				"GS": "small hail",
				"TS": "thunderstorm",
				"FG": "fog",
				"SQ": "squall",
				"FC": "funnel cloud",
			}

			if desc, ok := weatherMap[wxType]; ok {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("recent %s", desc),
				})
			} else {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: "recent weather phenomenon",
				})
			}
			i++
			continue
		}

		// Handle runway visual range (format: Rrrr/Vvvvft or similar)
		if strings.HasPrefix(part, "R") && strings.Contains(part, "/") {
			remarks = append(remarks, Remark{
				Raw:         part,
				Description: "runway visual range information",
			})
			i++
			continue
		}

		// Handle SNOINCR (format: SNINCR int/int)
		if part == "SNINCR" && i+1 < len(remarkParts) {
			snowData := remarkParts[i+1]
			if strings.Contains(snowData, "/") {
				parts := strings.Split(snowData, "/")
				if len(parts) == 2 {
					remarks = append(remarks, Remark{
						Raw:         part + " " + snowData,
						Description: fmt.Sprintf("snow increasing rapidly: %s inch within %s hour", parts[0], parts[1]),
					})
					i += 2
					continue
				}
			}
		}

		// Handle CIG for ceiling (format: CIG ddd)
		if part == "CIG" && i+1 < len(remarkParts) {
			if height, err := strconv.Atoi(remarkParts[i+1]); err == nil {
				remarks = append(remarks, Remark{
					Raw:         part + " " + remarkParts[i+1],
					Description: fmt.Sprintf("variable ceiling height: %d feet", height*100),
				})
				i += 2
				continue
			}
		}

		// Check for known remark codes
		if desc, ok := remarkCodes[part]; ok {
			remarks = append(remarks, Remark{
				Raw:         part,
				Description: desc,
			})
			i++
			continue
		}

		// Catch-all for unrecognized remarks
		remarks = append(remarks, Remark{
			Raw:         part,
			Description: "unknown remark code",
		})
		i++
	}

	return remarks
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
