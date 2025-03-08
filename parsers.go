package main

import (
	"fmt"
	"strconv"
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

// parseWind parses a wind string in the format "DDDSSKT" or "DDDSSGGKT"
func parseWind(windStr string) Wind {
	matches := windRegex.FindStringSubmatch(windStr)
	if matches == nil {
		return Wind{}
	}

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
