package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseTime parses a time string in the format "DDHHMM"Z
func parseTime(timeStr string) (time.Time, error) {
	matches := timeRegex.FindStringSubmatch(timeStr)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	day, _ := strconv.Atoi(matches[1])
	hour, _ := strconv.Atoi(matches[2])
	minute, _ := strconv.Atoi(matches[3])

	// Use current year and month
	now := time.Now().UTC()
	result := time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, time.UTC)

	// Handle month rollover
	if now.Day() < day {
		result = result.AddDate(0, -1, 0)
	}

	return result, nil
}

// parseWind parses a wind string in the format "DDDSSKT", "DDDSSGGKT", "DDDSSMPS", or "DDDSSGGMPS"
func parseWind(windStr string) Wind {
	// Try to match KT format first
	matches := windRegex.FindStringSubmatch(windStr)
	if matches != nil {
		wind := Wind{
			Direction: matches[1],
			Unit:      "KT",
		}

		wind.Speed, _ = strconv.Atoi(matches[2])
		if matches[4] != "" {
			wind.Gust, _ = strconv.Atoi(matches[4])
		}

		return wind
	}

	// Try to match MPS format
	matches = windRegexMPS.FindStringSubmatch(windStr)
	if matches != nil {
		wind := Wind{
			Direction: matches[1],
			Unit:      "MPS",
		}

		wind.Speed, _ = strconv.Atoi(matches[2])
		if matches[4] != "" {
			wind.Gust, _ = strconv.Atoi(matches[4])
		}

		return wind
	}

	return Wind{}
}

// parseWindVariation parses a wind variation string in the format "DDDVDDD"
func parseWindVariation(varStr string) string {
	matches := windVarRegex.FindStringSubmatch(varStr)
	if matches == nil {
		return ""
	}

	return varStr // Return the original string as is
}

// parseCloud parses a cloud string in the format "CCCHHH" or "CCCHHHTTT"
func parseCloud(cloudStr string) Cloud {
	matches := cloudRegex.FindStringSubmatch(cloudStr)
	if matches == nil {
		return Cloud{}
	}

	cloud := Cloud{
		Coverage: matches[1],
		Type:     matches[3],
	}

	// Only try to parse height if it exists
	if matches[2] != "" && len(matches[2]) > 0 {
		height, _ := strconv.Atoi(matches[2])
		cloud.Height = height * 100
	}

	return cloud
}

// parseRunwayCondition parses a runway condition string into a RunwayCondition struct
func parseRunwayCondition(condStr string) RunwayCondition {
	// Create a RunwayCondition with the raw string
	cond := RunwayCondition{Raw: condStr}

	// First check for CLRD (cleared) format
	if runwayClearedRegex.MatchString(condStr) {
		matches := runwayClearedRegex.FindStringSubmatch(condStr)
		if matches == nil || len(matches) < 3 {
			return cond
		}

		cond.Runway = matches[1]
		cond.Cleared = true
		clearedTime, _ := strconv.Atoi(matches[2])
		cond.ClearedTime = clearedTime
		return cond
	}

	// Then try the standard format
	matches := runwayCondRegex.FindStringSubmatch(condStr)
	// Needs at least runway and visibility value
	if matches == nil || len(matches) < 4 {
		return cond
	}

	// Extract runway identifier
	cond.Runway = matches[1]

	// Extract visibility and prefix
	visStr := matches[3]
	if len(visStr) > 0 {
		// Check for prefix (P for more than, M for less than)
		if visStr[0] == 'P' {
			cond.Prefix = "P"
			visStr = visStr[1:]
		} else if visStr[0] == 'M' {
			cond.Prefix = "M"
			visStr = visStr[1:]
		}

		// Parse visibility value
		vis, _ := strconv.Atoi(visStr)
		cond.Visibility = vis
	}

	// Check for variable visibility
	if matches[4] != "" {
		varVisStr := matches[5]

		// Handle prefixes in variable part
		if len(varVisStr) > 0 && (varVisStr[0] == 'P' || varVisStr[0] == 'M') {
			// Just remove the prefix for max visibility
			varVisStr = varVisStr[1:]
		}

		// Store min and max values
		cond.VisMin = cond.Visibility
		visMax, _ := strconv.Atoi(varVisStr)
		cond.VisMax = visMax
	}

	// Check for unit (FT for feet or nothing for meters)
	if matches[6] == "FT" {
		cond.Unit = "FT"
	}

	// Parse trend indicator
	if len(matches) > 7 && matches[7] != "" {
		// Extract the trend from either the full match or just the character
		if strings.HasPrefix(matches[7], "/") {
			// Format with slash: R21/1800V2000/U
			cond.Trend = matches[7][1:]
		} else {
			// Format without slash: R21/1800V2000U
			cond.Trend = matches[7]
		}
	}

	return cond
}

// parseForecastElement parses a single element of a forecast
func parseForecastElement(forecast *Forecast, part string) {
	// Wind
	if windRegex.MatchString(part) {
		forecast.Wind = parseWind(part)
		return
	}

	// Visibility
	if visRegexP.MatchString(part) || part == "P6SM" {
		forecast.Visibility = part
		return
	}

	// Clouds - check this BEFORE weather phenomena
	if cloudRegex.MatchString(part) {
		cloud := parseCloud(part)
		forecast.Clouds = append(forecast.Clouds, cloud)
		return
	}

	// Weather phenomena - check for weather codes
	if isWeatherCode(part) {
		forecast.Weather = append(forecast.Weather, part)
		return
	}
}
