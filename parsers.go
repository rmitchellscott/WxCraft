package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/utils/ptr"
)

// parseTime parses a "DDHHMMZ" time string, choosing the month and year that put it nearest to reference
func parseTime(timeStr string, reference time.Time) (time.Time, error) {
	matches := timeRegex.FindStringSubmatch(timeStr)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	day, _ := strconv.Atoi(matches[1])
	hour, _ := strconv.Atoi(matches[2])
	minute, _ := strconv.Atoi(matches[3])

	result, ok := resolveDayTime(reference, day, hour, minute)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid day/time: %s", timeStr)
	}
	return result, nil
}

// resolveDayTime places a day-of-month and time in the month (before, of, or after reference)
// that puts it nearest to reference. Hour 24 means midnight at the end of that day.
func resolveDayTime(reference time.Time, day, hour, minute int) (time.Time, bool) {
	if day < 1 || day > 31 || hour > 24 || minute > 59 || (hour == 24 && minute != 0) {
		return time.Time{}, false
	}

	reference = reference.UTC()
	monthStart := time.Date(reference.Year(), reference.Month(), 1, 0, 0, 0, 0, time.UTC)

	var best time.Time
	for _, monthOffset := range []int{-1, 0, 1} {
		candidateMonth := monthStart.AddDate(0, monthOffset, 0)
		dayStart := time.Date(candidateMonth.Year(), candidateMonth.Month(), day, 0, 0, 0, 0, time.UTC)
		if dayStart.Month() != candidateMonth.Month() {
			continue
		}
		candidate := dayStart.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
		if best.IsZero() || absDuration(candidate.Sub(reference)) < absDuration(best.Sub(reference)) {
			best = candidate
		}
	}
	return best, !best.IsZero()
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// parseWind parses a wind string in the format "DDDSSKT", "DDDSSGGKT", "DDDSSMPS", or "DDDSSGGMPS"
func parseWind(windStr string) Wind {
	if matches := zeroWindRegex.FindStringSubmatch(windStr); matches != nil {
		return Wind{
			Direction: matches[1],
			Speed:     ptr.To(0),
			Unit:      matches[3],
		}
	}

	// Try to match KT format
	matches := windRegex.FindStringSubmatch(windStr)
	if matches != nil {
		wind := Wind{
			Direction: matches[1],
			Unit:      "KT",
		}
		if matches[2] != "//" {
			speed, _ := strconv.Atoi(matches[2])
			wind.Speed = &speed
		}
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

		if matches[2] != "//" {
			speed, _ := strconv.Atoi(matches[2])
			wind.Speed = &speed
		}
		if matches[4] != "" {
			wind.Gust, _ = strconv.Atoi(matches[4])
		}

		return wind
	}

	return Wind{}
}

// parseCloud parses a cloud string in the format "CCCHHH", "CCCHHHTTT" or "CCCTTTHHH"
func parseCloud(cloudStr string) Cloud {
	matches := cloudRegex.FindStringSubmatch(cloudStr)
	if matches == nil {
		return Cloud{}
	}

	coverage, height, cloudType := matches[1], matches[2], matches[3]
	if coverage == "" {
		coverage, cloudType, height = matches[4], matches[5], matches[6]
	}

	cloud := Cloud{
		Coverage: coverage,
		Type:     cloudType,
	}

	switch {
	case height == "///":
		cloud.HeightMissing = true
	case height == "" && coverage != "SKC" && coverage != "CLR":
		cloud.HeightMissing = true
	case height != "":
		hundredsOfFeet, _ := strconv.Atoi(height)
		cloud.Height = hundredsOfFeet * 100
	}

	return cloud
}

// parseRunwayState parses a runway state group such as "R24/290050", "R88/CLRD//" or "R/SNOCLO"
func parseRunwayState(stateStr string) RunwayState {
	state := RunwayState{Raw: stateStr}

	if matches := runwaySnowClosedRegex.FindStringSubmatch(stateStr); matches != nil {
		state.Runway = matches[2]
		state.SnowClosed = true
		return state
	}

	if matches := runwayClearedRegex.FindStringSubmatch(stateStr); matches != nil {
		state.Runway = matches[1]
		state.Cleared = true
		state.Friction = matches[2]
		return state
	}

	if matches := runwayStateRegex.FindStringSubmatch(stateStr); matches != nil {
		state.Runway = matches[1]
		state.Deposit = matches[2]
		state.Extent = matches[3]
		state.Depth = matches[4]
		state.Friction = matches[5]
	}

	return state
}

// parseSeaState parses a sea surface group such as "W15/S2", "W///S9" or "W12/H7"
func parseSeaState(seaStr string) SeaState {
	sea := SeaState{Raw: seaStr}
	matches := seaStateRegex.FindStringSubmatch(seaStr)
	if matches == nil {
		return sea
	}

	if matches[1] != "//" {
		temperature, _ := strconv.Atoi(strings.TrimPrefix(matches[1], "M"))
		if strings.HasPrefix(matches[1], "M") {
			temperature = -temperature
		}
		sea.Temperature = &temperature
	}

	if matches[3] != "" && matches[3] != "/" {
		sea.State = matches[3]
	}

	if matches[4] != "" && !strings.HasPrefix(matches[4], "/") {
		waveHeight, _ := strconv.Atoi(matches[4])
		sea.WaveHeight = &waveHeight
	}

	return sea
}

// parseWindShear parses a wind shear string into a WindShear struct
func parseWindShear(wsStr string) WindShear {
	ws := WindShear{Raw: wsStr}

	if matches := windShearRwyRegex.FindStringSubmatch(wsStr); matches != nil {
		ws.Type = "RWY"
		ws.Phase = matches[1]
		ws.Runway = matches[2] + matches[3]
		return ws
	}

	// Try altitude wind shear format
	if matches := windShearAltRegex.FindStringSubmatch(wsStr); matches != nil {
		ws.Type = "ALT"
		altitude, _ := strconv.Atoi(matches[1])
		ws.Altitude = altitude

		// Parse wind at the shear level
		wind := Wind{
			Direction: matches[2],
			Unit:      "KT",
		}
		speed, _ := strconv.Atoi(matches[3])
		wind.Speed = &speed

		if matches[5] != "" {
			gust, _ := strconv.Atoi(matches[5])
			wind.Gust = gust
		}

		ws.Wind = wind
		return ws
	}

	return ws
}

// parseRunwayCondition parses a runway condition string into a RunwayCondition struct
func parseRunwayCondition(condStr string) RunwayCondition {
	// Create a RunwayCondition with the raw string
	cond := RunwayCondition{Raw: condStr}

	if matches := rvrMissingRegex.FindStringSubmatch(condStr); matches != nil {
		cond.Runway = strings.Trim(matches[1], "/")
		cond.Missing = true
		return cond
	}

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

// parseHazardLayer decodes the intensity, base (hundreds of feet), and thickness (thousands of feet)
// captured from a 6IhhhT or 5Bhhht group
func parseHazardLayer(matches []string, raw string) HazardLayer {
	base, _ := strconv.Atoi(matches[2])
	thickness, _ := strconv.Atoi(matches[3])
	return HazardLayer{
		Intensity:     matches[1],
		BaseFeet:      base * 100,
		ThicknessFeet: thickness * 1000,
		Raw:           raw,
	}
}
