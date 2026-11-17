package main

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// DecodeTAF decodes a raw TAF string into a TAF struct. Day-of-month times in the report are
// placed in the month nearest to now, then period times in the month nearest to the issue time.
func DecodeTAF(raw string, now time.Time) TAF {
	t := TAF{WeatherData: WeatherData{Raw: raw}}
	parts := strings.Fields(raw)

	i := 0
	if i < len(parts) && parts[i] == "TAF" {
		i++
	}
	for ; i < len(parts); i++ {
		if parts[i] == "AMD" {
			t.Amended = true
		} else if parts[i] == "COR" {
			t.Corrected = true
		} else {
			break
		}
	}
	if i >= len(parts) {
		return t
	}

	t.Station = parts[i]
	t.SiteInfo = SiteInfo{Name: t.Station}
	i++

	reference := now
	if i < len(parts) && (timeRegex.MatchString(parts[i]) || tafDayTimeRegex.MatchString(parts[i])) {
		if issued, err := parseTime(strings.TrimSuffix(parts[i], "Z")+"Z", now); err == nil {
			t.Time = issued
			reference = issued
		}
		i++
	}

	if i < len(parts) && parts[i] == "NIL" {
		t.Nil = true
		i++
	}

	if i < len(parts) {
		if from, to, ok := parsePeriod(parts[i], reference); ok {
			t.ValidFrom, t.ValidTo = from, to
			if t.Time.IsZero() {
				reference = from
			}
			i++
		}
	}

	if i < len(parts) && parts[i] == "CNL" {
		t.Cancelled = true
		i++
	}

	body := parts[i:]
	for j, part := range body {
		if part == "RMK" {
			t.Remarks = strings.Join(body[j+1:], " ")
			body = body[:j]
			break
		}
	}

	if t.Nil || t.Cancelled {
		return t
	}

	current := Forecast{Type: "BASE", From: t.ValidFrom}
	periodStart := 0
	finishPeriod := func(end int) {
		current.Raw = strings.Join(body[periodStart:end], " ")
		t.Forecasts = append(t.Forecasts, current)
	}

	for j := 0; j < len(body); {
		if next, consumed, ok := parseChangeGroup(body, j, reference); ok {
			finishPeriod(j)
			if next.Type == "FM" {
				endOpenPeriod(t.Forecasts, next.From)
			}
			if next.From.IsZero() {
				next.From, next.To = t.ValidFrom, t.ValidTo
			}
			current = next
			periodStart = j
			j += consumed
			continue
		}

		part := body[j]
		if j+1 < len(body) && tafVisWholeRegex.MatchString(part) && tafVisFractionRegex.MatchString(body[j+1]) {
			current.Visibility = part + " " + body[j+1]
			j += 2
			continue
		}

		if matches := tafTemperatureRegex.FindStringSubmatch(part); matches != nil {
			if temperature, ok := parseTemperatureForecast(matches, reference); ok {
				t.Temperatures = append(t.Temperatures, temperature)
				j++
				continue
			}
		}

		if !parseForecastElement(&current, part) {
			current.Unhandled = append(current.Unhandled, part)
		}
		j++
	}
	finishPeriod(len(body))

	endOpenPeriod(t.Forecasts, t.ValidTo)

	return t
}

// parseChangeGroup recognizes a change group header (FMddhhmm, FM ddhhmm, BECMG, TEMPO, INTER, or
// PROBnn optionally followed by TEMPO or INTER, then an optional ddhh/ddhh period) at body[i]
func parseChangeGroup(body []string, i int, reference time.Time) (Forecast, int, bool) {
	part := body[i]

	if matches := tafFromRegex.FindStringSubmatch(part); matches != nil {
		return fromForecast(matches, 1, reference)
	}
	if part == "FM" && i+1 < len(body) {
		if matches := tafDayTimeRegex.FindStringSubmatch(body[i+1]); matches != nil {
			return fromForecast(matches, 2, reference)
		}
	}

	var forecast Forecast
	consumed := 1
	switch {
	case part == "BECMG" || part == "TEMPO" || part == "INTER":
		forecast.Type = part
	case tafProbRegex.MatchString(part):
		matches := tafProbRegex.FindStringSubmatch(part)
		forecast.Probability, _ = strconv.Atoi(matches[1])
		forecast.Type = "PROB"
		if matches[2] != "" {
			forecast.Type = matches[2]
		} else if i+1 < len(body) && (body[i+1] == "TEMPO" || body[i+1] == "INTER") {
			forecast.Type = body[i+1]
			consumed++
		}
	default:
		return Forecast{}, 0, false
	}

	if i+consumed < len(body) {
		if from, to, ok := parsePeriod(body[i+consumed], reference); ok {
			forecast.From, forecast.To = from, to
			consumed++
		}
	}
	return forecast, consumed, true
}

func fromForecast(matches []string, consumed int, reference time.Time) (Forecast, int, bool) {
	day, _ := strconv.Atoi(matches[1])
	hour, _ := strconv.Atoi(matches[2])
	minute, _ := strconv.Atoi(matches[3])
	from, ok := resolveDayTime(reference, day, hour, minute)
	if !ok {
		return Forecast{}, 0, false
	}
	return Forecast{Type: "FM", From: from}, consumed, true
}

// parsePeriod decodes a ddhh/ddhh validity or change period
func parsePeriod(part string, reference time.Time) (time.Time, time.Time, bool) {
	matches := validRegex.FindStringSubmatch(part)
	if matches == nil {
		return time.Time{}, time.Time{}, false
	}
	fromDay, _ := strconv.Atoi(matches[1])
	fromHour, _ := strconv.Atoi(matches[2])
	toDay, _ := strconv.Atoi(matches[3])
	toHour, _ := strconv.Atoi(matches[4])

	from, fromOK := resolveDayTime(reference, fromDay, fromHour, 0)
	if !fromOK {
		return time.Time{}, time.Time{}, false
	}
	to, toOK := resolveDayTime(from, toDay, toHour, 0)
	if !toOK || to.Before(from) {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

// endOpenPeriod closes the most recent BASE or FM period that has no end time; TEMPO, BECMG,
// INTER, and PROB periods overlay those and do not end them
func endOpenPeriod(forecasts []Forecast, end time.Time) {
	for k := len(forecasts) - 1; k >= 0; k-- {
		if forecasts[k].Type == "BASE" || forecasts[k].Type == "FM" {
			if forecasts[k].To.IsZero() {
				forecasts[k].To = end
			}
			return
		}
	}
}

func parseTemperatureForecast(matches []string, reference time.Time) (TemperatureForecast, bool) {
	celsius, _ := strconv.Atoi(matches[3])
	if matches[2] == "M" {
		celsius = -celsius
	}
	day, _ := strconv.Atoi(matches[4])
	hour, _ := strconv.Atoi(matches[5])
	at, ok := resolveDayTime(reference, day, hour, 0)
	if !ok {
		return TemperatureForecast{}, false
	}
	kind := map[string]string{"X": "MAX", "N": "MIN"}[matches[1]]
	return TemperatureForecast{Kind: kind, Celsius: celsius, At: at, Raw: matches[0]}, true
}

// isWeatherCode reports whether s is a present or recent weather group
func isWeatherCode(s string) bool {
	matches := weatherGroupRegex.FindStringSubmatch(s)
	if matches == nil {
		return false
	}
	descriptor, phenomena := matches[4], matches[5]
	return phenomena != "" || descriptor == "TS" || descriptor == "SH"
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

	stationIndex := 0
	if len(parts) > 0 && (parts[0] == "METAR" || parts[0] == "SPECI") {
		m.ReportType = parts[0]
		stationIndex++
	}
	if stationIndex < len(parts) && parts[stationIndex] == "COR" {
		m.SpecialCodes = append(m.SpecialCodes, "COR")
		stationIndex++
	}

	if len(parts) < stationIndex+2 {
		return m
	}

	// Station code
	m.Station = parts[stationIndex]

	// Initialize default site info
	m.SiteInfo = SiteInfo{
		Name:    m.Station,
		State:   "",
		Country: "",
	}

	// Time
	bodyStart := stationIndex + 1
	if timeRegex.MatchString(parts[bodyStart]) {
		if parsedTime, err := parseTime(parts[bodyStart], time.Now().UTC()); err == nil {
			m.Time = parsedTime
		}
		bodyStart++
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
	for i := bodyStart; i < endIndex-1; i++ {
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

	for i := bodyStart; i < endIndex; i++ {
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

		// US automated stations report a missing group as a bare "M"
		if part == "M" {
			if !slices.Contains(m.SpecialCodes, part) {
				m.SpecialCodes = append(m.SpecialCodes, part)
			}
			continue
		}

		if militaryColorRegex.MatchString(part) {
			m.SpecialCodes = append(m.SpecialCodes, part)
			continue
		}

		// Wind - check both KT and MPS formats
		if windRegex.MatchString(part) || windRegexMPS.MatchString(part) {
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

		if part == "////" || part == "////SM" {
			if m.Visibility == "" {
				m.Visibility = "////"
			}
			continue
		}

		if m.Visibility != "" && visRegexDir.MatchString(part) {
			m.MinVisibility = part
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

		if part == "VV///" {
			m.VertVisMissing = true
			continue
		}

		// Runway state must be tried before RVR: R24/290050 also matches the RVR pattern
		if runwayStateRegex.MatchString(part) || runwayClearedRegex.MatchString(part) || runwaySnowClosedRegex.MatchString(part) {
			m.RunwayStates = append(m.RunwayStates, parseRunwayState(part))
			continue
		}

		if runwayCondRegex.MatchString(part) || rvrMissingRegex.MatchString(part) {
			m.RunwayConditions = append(m.RunwayConditions, parseRunwayCondition(part))
			continue
		}

		// "//" and "RE//": present or recent weather not observable by the automated station
		if isWeatherCode(part) || part == "//" || part == "RE//" {
			m.Weather = append(m.Weather, part)
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
			// Store temperature as a pointer to int
			m.Temperature = &temp

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
			// Store temperature as a pointer to int
			m.Temperature = &temp
			// Leave DewPoint as nil to indicate missing value
			continue
		}

		if part == "/////" {
			continue
		}

		if pressureMissingRegex.MatchString(part) {
			if !pressureFound {
				m.PressureUnit = "hPa"
				if part[0] == 'A' {
					m.PressureUnit = "inHg"
				}
				pressureFound = true
			}
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

		if seaStateRegex.MatchString(part) {
			m.SeaStates = append(m.SeaStates, parseSeaState(part))
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
