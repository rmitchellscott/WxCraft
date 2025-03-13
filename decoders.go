package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DecodeTAF decodes a raw TAF string into a TAF struct
func DecodeTAF(raw string) TAF {
	t := TAF{WeatherData: WeatherData{Raw: raw}}

	// Remove line breaks and consolidate whitespace
	cleanedRaw := strings.TrimSpace(raw)
	cleanedRaw = regexp.MustCompile(`\s+`).ReplaceAllString(cleanedRaw, " ")

	// Split into parts
	parts := strings.Fields(cleanedRaw)
	if len(parts) < 3 {
		return t
	}

	// Check for TAF indicator and extract station
	startIdx := 0
	if parts[0] == "TAF" {
		startIdx = 1
		t.Station = parts[1]
	} else {
		t.Station = parts[0]
	}
	// Initialize default site info
	t.SiteInfo = SiteInfo{
		Name:    t.Station,
		State:   "",
		Country: "",
	}

	// Parse issuance time
	for i := startIdx + 1; i < len(parts); i++ {
		if timeRegex.MatchString(parts[i]) {
			if parsedTime, err := parseTime(parts[i]); err == nil {
				t.Time = parsedTime
			}
			continue
		}

		// Parse valid time period
		if validRegex.MatchString(parts[i]) {
			matches := validRegex.FindStringSubmatch(parts[i])
			fromDay, _ := strconv.Atoi(matches[1])
			fromHour, _ := strconv.Atoi(matches[2])
			toDay, _ := strconv.Atoi(matches[3])
			toHour, _ := strconv.Atoi(matches[4])

			// Use current year and month
			now := time.Now().UTC()
			t.ValidFrom = time.Date(now.Year(), now.Month(), fromDay, fromHour, 0, 0, 0, time.UTC)
			t.ValidTo = time.Date(now.Year(), now.Month(), toDay, toHour, 0, 0, 0, time.UTC)
			break
		}
	}

	// Create base forecast from the main TAF line
	baseForecast := Forecast{
		Type: "BASE",
		From: t.ValidFrom,
		Raw:  cleanedRaw,
	}

	// Find index of first FM, BECMG, TEMPO, or PROB
	var changeIndex int
	for i, part := range parts {
		if part == "FM" || strings.HasPrefix(part, "FM") ||
			part == "BECMG" || part == "TEMPO" ||
			strings.HasPrefix(part, "PROB") {
			changeIndex = i
			break
		}

		if i >= len(parts)-1 {
			changeIndex = len(parts)
		}
	}

	// Parse elements for base forecast
	if changeIndex > 0 {
		validTimeRegex := regexp.MustCompile(`\d{4}/\d{4}`)
		for i := startIdx + 1; i < changeIndex; i++ {
			part := parts[i]
			if timeRegex.MatchString(part) || validTimeRegex.MatchString(part) {
				continue
			}
			parseForecastElement(&baseForecast, part)
		}
	}

	t.Forecasts = append(t.Forecasts, baseForecast)

	// Process change groups
	i := changeIndex
	for i < len(parts) {
		part := parts[i]

		if part == "FM" || strings.HasPrefix(part, "FM") {
			forecast := Forecast{
				Type: "FM",
				Raw:  part,
			}

			// Parse FM time
			var fmTime string
			if part == "FM" && i+1 < len(parts) {
				fmTime = parts[i+1]
				i++
			} else if strings.HasPrefix(part, "FM") {
				fmTime = part[2:]
			}

			if len(fmTime) == 6 {
				day, _ := strconv.Atoi(fmTime[0:2])
				hour, _ := strconv.Atoi(fmTime[2:4])
				minute, _ := strconv.Atoi(fmTime[4:6])

				// Use current year and month
				now := time.Now().UTC()
				forecast.From = time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, time.UTC)

				// Handle month rollover
				if now.Day() > day {
					forecast.From = forecast.From.AddDate(0, 1, 0)
				}
			}

			// Set the To time of the previous forecast if it needs it
			if len(t.Forecasts) > 0 && t.Forecasts[len(t.Forecasts)-1].To.IsZero() {
				t.Forecasts[len(t.Forecasts)-1].To = forecast.From
			}

			// Parse elements until next change indicator
			i++
			for i < len(parts) {
				nextPart := parts[i]
				if nextPart == "FM" || strings.HasPrefix(nextPart, "FM") ||
					nextPart == "BECMG" || nextPart == "TEMPO" ||
					strings.HasPrefix(nextPart, "PROB") {
					break
				}
				parseForecastElement(&forecast, nextPart)
				i++
			}

			t.Forecasts = append(t.Forecasts, forecast)
			continue
		}

		if part == "BECMG" || part == "TEMPO" {
			forecast := Forecast{
				Type: part,
				Raw:  part,
			}

			// Parse time period if available
			i++
			if i < len(parts) {
				timeRegex := regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
				if timeRegex.MatchString(parts[i]) {
					matches := timeRegex.FindStringSubmatch(parts[i])
					fromDay, _ := strconv.Atoi(matches[1])
					fromHour, _ := strconv.Atoi(matches[2])
					toDay, _ := strconv.Atoi(matches[3])
					toHour, _ := strconv.Atoi(matches[4])

					// Use current year and month
					now := time.Now().UTC()
					forecast.From = time.Date(now.Year(), now.Month(), fromDay, fromHour, 0, 0, 0, time.UTC)
					forecast.To = time.Date(now.Year(), now.Month(), toDay, toHour, 0, 0, 0, time.UTC)
					i++
				}
			}

			// Parse elements until next change indicator
			for i < len(parts) {
				nextPart := parts[i]
				if nextPart == "FM" || strings.HasPrefix(nextPart, "FM") ||
					nextPart == "BECMG" || nextPart == "TEMPO" ||
					strings.HasPrefix(nextPart, "PROB") {
					break
				}
				parseForecastElement(&forecast, nextPart)
				i++
			}

			t.Forecasts = append(t.Forecasts, forecast)
			continue
		}

		// Handle PROB30 and PROB40 forecasts
		if strings.HasPrefix(part, "PROB") {
			probValue, err := strconv.Atoi(part[4:])
			if err != nil {
				// If we can't parse the probability value, skip this part
				i++
				continue
			}

			forecast := Forecast{
				Type:        part,
				Probability: probValue,
				Raw:         part,
			}

			// Parse time period if available
			i++
			if i < len(parts) {
				timeRegex := regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
				if timeRegex.MatchString(parts[i]) {
					matches := timeRegex.FindStringSubmatch(parts[i])
					fromDay, _ := strconv.Atoi(matches[1])
					fromHour, _ := strconv.Atoi(matches[2])
					toDay, _ := strconv.Atoi(matches[3])
					toHour, _ := strconv.Atoi(matches[4])

					// Use current year and month
					now := time.Now().UTC()
					forecast.From = time.Date(now.Year(), now.Month(), fromDay, fromHour, 0, 0, 0, time.UTC)
					forecast.To = time.Date(now.Year(), now.Month(), toDay, toHour, 0, 0, 0, time.UTC)
					i++
				}
			}

			// If no time period is given, use the current valid period
			if forecast.From.IsZero() {
				// Try to use the time from the most recent forecast
				if len(t.Forecasts) > 0 {
					lastForecast := t.Forecasts[len(t.Forecasts)-1]
					forecast.From = lastForecast.From
					forecast.To = lastForecast.To
				} else {
					forecast.From = t.ValidFrom
					forecast.To = t.ValidTo
				}
			}

			// Parse elements until next change indicator
			for i < len(parts) {
				nextPart := parts[i]
				if nextPart == "FM" || strings.HasPrefix(nextPart, "FM") ||
					nextPart == "BECMG" || nextPart == "TEMPO" ||
					strings.HasPrefix(nextPart, "PROB") {
					break
				}
				parseForecastElement(&forecast, nextPart)
				i++
			}

			t.Forecasts = append(t.Forecasts, forecast)
			continue
		}

		i++
	}

	// Set the final forecast's To time if needed
	if len(t.Forecasts) > 0 && t.Forecasts[len(t.Forecasts)-1].To.IsZero() {
		t.Forecasts[len(t.Forecasts)-1].To = t.ValidTo
	}

	return t
}

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

// isVisibilityInMeters checks if a string is a visibility value in meters
func isVisibilityInMeters(s string) bool {
	// Basic check for 4-digit number (standard visibility in meters)
	if visRegexNum.MatchString(s) {
		return true
	}

	// Check for visibility with direction (e.g., "2000NE")
	if visRegexDir.MatchString(s) {
		return true
	}

	// Check for visibility with No Directional Variation (e.g., "4000NDV")
	if ndvRegex.MatchString(s) {
		return true
	}

	// Special case for visibility less than 50m reported as "0000"
	if s == "0000" {
		return true
	}

	return false
}

// isVerticalVisibility checks if a string is a vertical visibility value
func isVerticalVisibility(s string) bool {
	return vvRegex.MatchString(s)
}

// DecodeMETAR decodes a raw METAR string into a METAR struct with site information
func DecodeMETAR(raw string) METAR {
	m := METAR{WeatherData: WeatherData{Raw: raw}}
	parts := strings.Fields(raw)

	if len(parts) < 2 {
		return m
	}

	// Station code
	m.Station = parts[0]

	// Initialize default site info
	m.SiteInfo = SiteInfo{
		Name:    m.Station,
		State:   "",
		Country: "",
	}

	// Time
	if timeRegex.MatchString(parts[1]) {
		if parsedTime, err := parseTime(parts[1]); err == nil {
			m.Time = parsedTime
		}
	}

	// Find the RMK section, BECMG section, and TEMPO section if they exist
	rmkIndex := -1
	sectionIndices := []int{}

	// Find all TEMPO, BECMG, and RMK sections
	for i, part := range parts {
		if part == "RMK" {
			rmkIndex = i
			break // RMK always ends the main section
		}
		if part == "TEMPO" || part == "BECMG" || part == "INTER" {
			sectionIndices = append(sectionIndices, i)
		}
	}

	// Find the first section marker
	endIndex := len(parts)
	if rmkIndex != -1 {
		endIndex = rmkIndex
	}

	// Find the earliest TEMPO or BECMG section
	for _, idx := range sectionIndices {
		if idx < endIndex {
			endIndex = idx
		}
	}

	// Process fields in the main METAR section
	// Variable to track if we've already found a pressure value
	pressureFound := false

	// THIS IS GROSS, I DON'T LIKE IT
	// Special handling for split wind shear tokens
	for i := 2; i < endIndex-1; i++ {
		// Handle "WS ALL RWY" pattern
		if i+2 < endIndex && parts[i] == "WS" && parts[i+1] == "ALL" && parts[i+2] == "RWY" {
			// Found the three-token pattern for wind shear
			m.WindShear = append(m.WindShear, WindShear{
				Type:  "RWY",
				Phase: "ALL",
				Raw:   "WS ALL RWY",
			})
			// Skip these tokens in the main loop
			parts[i] = "__PROCESSED__"
			parts[i+1] = "__PROCESSED__"
			parts[i+2] = "__PROCESSED__"
			// Handle "WS R##" pattern
		} else if i+1 < endIndex && parts[i] == "WS" && strings.HasPrefix(parts[i+1], "R") && len(parts[i+1]) > 1 {
			// Found the two-token pattern for wind shear on a runway
			m.WindShear = append(m.WindShear, WindShear{
				Type:   "RWY",
				Runway: parts[i+1][1:], // Remove the 'R' prefix
				Raw:    parts[i] + " " + parts[i+1],
			})
			// Skip these tokens in the main loop
			parts[i] = "__PROCESSED__"
			parts[i+1] = "__PROCESSED__"
		}
	}

	for i := 2; i < endIndex; i++ {
		part := parts[i]

		// Skip tokens that were already processed
		if part == "__PROCESSED__" {
			continue
		}

		// Special conditions (AUTO, COR, etc.)
		if specialRegex.MatchString(part) {
			m.SpecialCodes = append(m.SpecialCodes, part)
			continue
		}

		// Wind
		if windRegex.MatchString(part) {
			m.Wind = parseWind(part)

			// Check if the next token is a wind variation
			if i+1 < endIndex && windVarRegex.MatchString(parts[i+1]) {
				m.WindVariation = parseWindVariation(parts[i+1])
				i++ // Skip the next token since we've processed it
			}

			continue
		}
		// Check for wind shear
		if strings.HasPrefix(part, "WS") {
			ws := parseWindShear(part)
			m.WindShear = append(m.WindShear, ws)
			continue
		}

		// Visibility - handle special cases like "1 1/2SM" (split across two tokens)
		if i+1 < endIndex && strings.HasSuffix(parts[i+1], "SM") &&
			!strings.HasPrefix(parts[i], "P") && !strings.HasPrefix(parts[i], "M") &&
			!strings.Contains(parts[i], "/") && len(parts[i]) == 1 {
			// This could be a split visibility value like "1 1/2SM"
			m.Visibility = parts[i] + " " + parts[i+1]
			i++ // Skip the next token since we've processed it
			continue
		}

		// Standard visibility check for statute miles
		if visRegexM.MatchString(part) {
			m.Visibility = part
			continue
		}

		// Check for visibility in meters
		if isVisibilityInMeters(part) {
			m.Visibility = part
			continue
		}

		// Check for vertical visibility (e.g., "VV002")
		if isVerticalVisibility(part) {
			matches := vvRegex.FindStringSubmatch(part)
			if len(matches) > 1 {
				vertVis, _ := strconv.Atoi(matches[1])
				m.VertVis = vertVis
			}
			continue
		}

		// Runway Visual Range (RVR) and Runway Conditions
		if runwayClearedRegex.MatchString(part) || runwayCondRegex.MatchString(part) {
			// Parse the runway condition
			cond := parseRunwayCondition(part)
			m.RunwayConditions = append(m.RunwayConditions, cond)
			// Add to legacy RVR field for compatibility
			m.RVR = append(m.RVR, part)
			continue
		}

		// Basic RVR format (legacy)
		if rvrRegex.MatchString(part) {
			m.RVR = append(m.RVR, part)
			continue
		}

		// Weather phenomena
		if isWeatherCode(part) {
			m.Weather = append(m.Weather, part)
			continue
		}

		// Wind shear
		if strings.Contains(part, "WS") {
			ws := parseWindShear(part)
			m.WindShear = append(m.WindShear, ws)
			continue
		}

		// Clouds
		if cloudRegex.MatchString(part) {
			cloud := parseCloud(part)
			m.Clouds = append(m.Clouds, cloud)
			continue
		}

		// Temperature and dew point - standard format
		if tempRegex.MatchString(part) {
			matches := tempRegex.FindStringSubmatch(part)
			temp, _ := strconv.Atoi(matches[2])
			if matches[1] == "M" {
				temp = -temp
			}
			m.Temperature = temp

			dewPoint, _ := strconv.Atoi(matches[4])
			if matches[3] == "M" {
				dewPoint = -dewPoint
			}
			// Store dew point as a pointer to int
			m.DewPoint = &dewPoint
			continue
		}

		// Temperature only format "M01/" (missing dew point)
		if tempOnlyRegex.MatchString(part) {
			matches := tempOnlyRegex.FindStringSubmatch(part)
			temp, _ := strconv.Atoi(matches[2])
			if matches[1] == "M" {
				temp = -temp
			}
			m.Temperature = temp
			// Leave DewPoint as nil to indicate missing value
			continue
		}

		// Pressure in Q format (hPa/millibars) - only process if we haven't found pressure yet
		if len(part) > 1 && part[0] == 'Q' {
			if pressureFound {
				continue
			}
			pressureStr := part[1:]
			pressureInt, err := strconv.Atoi(pressureStr)
			if err == nil {
				m.Pressure = float64(pressureInt)
				m.PressureUnit = "hPa"
				pressureFound = true
			}
			continue
		}

		// Pressure in A format (inches of mercury) - only process if we haven't found pressure yet
		if pressureRegex.MatchString(part) {
			if pressureFound {
				continue
			}
			matches := pressureRegex.FindStringSubmatch(part)
			pressureStr := matches[1]
			pressureInt, _ := strconv.Atoi(pressureStr)
			m.Pressure = float64(pressureInt) / 100.0
			m.PressureUnit = "inHg"
			pressureFound = true
			continue
		}

		// CAVOK - Ceiling And Visibility OK
		if cavokRegex.MatchString(part) {
			m.Visibility = "CAVOK"
			m.SpecialCodes = append(m.SpecialCodes, "CAVOK")
			continue
		}

		m.Unhandled = append(m.Unhandled, part)
	}

	// Process remarks if they exist
	if rmkIndex != -1 && rmkIndex+1 < len(parts) {
		m.Remarks = processRemarks(parts[rmkIndex+1:])
	}

	return m
}
