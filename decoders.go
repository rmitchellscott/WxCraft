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

	forecastEnd := len(body)
	for j, part := range body {
		if tafPlainLanguageStarts[part] {
			forecastEnd = j
			break
		}
	}

	baseTokens, changeGroups := splitChangeGroups(body[:forecastEnd], reference)
	base := Forecast{Type: "BASE", From: t.ValidFrom, Raw: strings.Join(baseTokens, " ")}
	decoded := decodeGroups(baseTokens, periodGrammar, reference)
	decoded.applyToForecast(&base)
	t.Forecasts = append(t.Forecasts, base)
	t.Temperatures = append(t.Temperatures, decoded.temperatures...)

	for _, changeGroup := range changeGroups {
		period, decoded := changeGroup.decodePeriod(reference)
		if period.Type == "FM" {
			endOpenPeriod(t.Forecasts, period.From)
		}
		if period.From.IsZero() {
			period.From, period.To = t.ValidFrom, t.ValidTo
		}
		t.Forecasts = append(t.Forecasts, period)
		t.Temperatures = append(t.Temperatures, decoded.temperatures...)
	}

	trailer := decodeGroups(body[forecastEnd:], []group{forecastTemperatureGroup}, reference)
	t.Temperatures = append(t.Temperatures, trailer.temperatures...)
	if len(trailer.unhandled) > 0 {
		t.Remarks = strings.TrimSpace(strings.Join(trailer.unhandled, " ") + " " + t.Remarks)
	}

	endOpenPeriod(t.Forecasts, t.ValidTo)
	markPeriodsInEffect(t.Forecasts, now)

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
	for i+consumed < len(body) && applyTrendTime(&forecast, body[i+consumed], reference) {
		consumed++
	}
	return forecast, consumed, true
}

// applyTrendTime sets the FM, TL, or AT hhmm time of a METAR trend group, placed at or after
// reference, the observation time
func applyTrendTime(forecast *Forecast, part string, reference time.Time) bool {
	matches := trendTimeRegex.FindStringSubmatch(part)
	if matches == nil {
		return false
	}
	hour, _ := strconv.Atoi(matches[2])
	minute, _ := strconv.Atoi(matches[3])
	if hour > 24 || minute > 59 {
		return false
	}

	reference = reference.UTC()
	at := time.Date(reference.Year(), reference.Month(), reference.Day(), hour, minute, 0, 0, time.UTC)
	if at.Before(reference) {
		at = at.AddDate(0, 0, 1)
	}
	switch matches[1] {
	case "FM":
		forecast.From = at
	case "TL":
		forecast.To = at
	case "AT":
		forecast.At = at
	}
	return true
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

// DecodeMETAR decodes a raw METAR string into a METAR struct with site information
func DecodeMETAR(raw string, now time.Time) METAR {
	m := METAR{WeatherData: WeatherData{Raw: raw}}
	var parts []string
	for _, part := range strings.Fields(raw) {
		// A few stations omit the space between the pressure group and the next section, as in "Q1006TEMPO"
		if matches := gluedPressureRegex.FindStringSubmatch(part); matches != nil {
			parts = append(parts, matches[1], matches[2])
		} else {
			parts = append(parts, part)
		}
	}

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
		if parsedTime, err := parseTime(parts[bodyStart], now); err == nil {
			m.Time = parsedTime
		}
		bodyStart++
	}

	bodyEnd := len(parts)
	if rmkIndex := slices.Index(parts[bodyStart:], "RMK"); rmkIndex != -1 {
		bodyEnd = bodyStart + rmkIndex
		if bodyEnd+1 < len(parts) {
			m.Remarks = processRemarks(parts[bodyEnd+1:])
		}
	}

	reference := now
	if !m.Time.IsZero() {
		reference = m.Time
	}
	bodyTokens, trends := splitChangeGroups(parts[bodyStart:bodyEnd], reference)
	decodeGroups(bodyTokens, metarBodyGrammar, reference).applyToMETAR(&m)
	for _, trend := range trends {
		forecast, _ := trend.decodePeriod(reference)
		m.Trends = append(m.Trends, forecast)
	}

	return m
}
