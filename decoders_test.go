package main

import (
	"iter"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rmitchellscott/WxCraft/testdata"
	"github.com/stretchr/testify/assert"
)

func decodeMETARList(t *testing.T) iter.Seq2[string, METAR] {
	return func(yield func(string, METAR) bool) {
		scanner := testdata.METAR(t)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "//") {
				continue
			}
			line := strings.TrimSpace(scanner.Text())
			if !yield(line, DecodeMETAR(line)) {
				return
			}
		}
	}
}

func TestDecodeMETAR_stationCode(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		assert.Equal(t, fields[0], metar.SiteInfo.Name, line, "station code did not match")
	}
}

func TestDecodeMETAR_time(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		if fields[1] != "COR" {
			got := metar.Time.Format("021504") + "Z"
			assert.Equal(t, fields[1], got, line, "time did not match")
		}
	}
}

func TestDecodeMETAR_remarks(t *testing.T) {
	t.Parallel()

	var unknownRemarks []string
	var failedRemarkCount int

	for line, metar := range decodeMETARList(t) {
		var unknown []string
		for _, rmk := range metar.Remarks {
			if rmk.Description == "unknown remark code" {
				unknown = append(unknown, rmk.Raw)
				unknownRemarks = append(unknownRemarks, rmk.Raw)
			}
		}

		if len(unknown) != 0 {
			failedRemarkCount++
			t.Errorf("Unknown remarks:\nMETAR   = %s\nRemarks = %v", line, unknown)
		}
	}

	t.Run("unknown remark count", func(t *testing.T) {
		slices.Sort(unknownRemarks)
		unknownRemarks = slices.Compact(unknownRemarks)
		assert.Empty(t, len(unknownRemarks))
	})

	t.Run("metars with failed remarks", func(t *testing.T) {
		assert.Zero(t, failedRemarkCount)
	})
}

func TestDecodeMETAR_visibility(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		for _, field := range fields[1:] {
			if strings.HasSuffix(field, "SM") {
				assert.Equal(t, field, metar.Visibility)
			}
		}
	}
}

func TestDecodeMETAR_wind(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		for _, field := range fields[1:] {
			if windRegex.MatchString(field) {
				expectedWind := parseWind(field)
				assert.Equal(t, expectedWind, metar.Wind, line, "wind parsed incorrectly")
				break
			}
		}
	}
}

func TestDecodeMETAR_weather(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedWeather []string

		// Find sections to know where to stop
		rmkIndex := -1
		becmgIndex := -1
		tempoIndex := -1
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break
			}
			if part == "BECMG" {
				becmgIndex = i
				break
			}
			if part == "TEMPO" {
				tempoIndex = i
				break
			}
		}

		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}
		if becmgIndex != -1 && (rmkIndex == -1 || becmgIndex < rmkIndex) {
			endIndex = becmgIndex
		}
		if tempoIndex != -1 && tempoIndex < endIndex {
			endIndex = tempoIndex
		}

		// Collect weather phenomena from original METAR
		for i := 2; i < endIndex; i++ {
			if isWeatherCode(fields[i]) {
				expectedWeather = append(expectedWeather, fields[i])
			}
		}

		// Check if weather phenomena were parsed correctly
		assert.ElementsMatch(t, expectedWeather, metar.Weather, line, "weather phenomena parsed incorrectly")
	}
}

func TestDecodeMETAR_clouds(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedClouds []Cloud

		// Find sections to know where to stop
		rmkIndex := -1
		becmgIndex := -1
		tempoIndex := -1
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break
			}
			if part == "BECMG" {
				becmgIndex = i
				break
			}
			if part == "TEMPO" {
				tempoIndex = i
				break
			}
		}

		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}
		if becmgIndex != -1 && (rmkIndex == -1 || becmgIndex < rmkIndex) {
			endIndex = becmgIndex
		}
		if tempoIndex != -1 && tempoIndex < endIndex {
			endIndex = tempoIndex
		}

		// Collect cloud data from original METAR
		for i := 2; i < endIndex; i++ {
			if cloudRegex.MatchString(fields[i]) {
				expectedClouds = append(expectedClouds, parseCloud(fields[i]))
			}
		}

		// Check if clouds were parsed correctly
		assert.Equal(t, len(expectedClouds), len(metar.Clouds), line, "wrong number of cloud layers")
		for i := range expectedClouds {
			if i < len(metar.Clouds) {
				assert.Equal(t, expectedClouds[i], metar.Clouds[i], line, "cloud layer parsed incorrectly")
			}
		}
	}
}

func TestDecodeMETAR_temperature(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find temperature/dew point in the original METAR
		for _, field := range fields {
			if tempRegex.MatchString(field) {
				matches := tempRegex.FindStringSubmatch(field)
				expectedTemp, _ := strconv.Atoi(matches[2])
				if matches[1] == "M" {
					expectedTemp = -expectedTemp
				}

				expectedDew, _ := strconv.Atoi(matches[4])
				if matches[3] == "M" {
					expectedDew = -expectedDew
				}

				assert.Equal(t, expectedTemp, metar.Temperature, line, "temperature parsed incorrectly")
				assert.Equal(t, expectedDew, metar.DewPoint, line, "dew point parsed incorrectly")
				break
			}
		}
	}
}

func TestDecodeMETAR_pressure(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Skip the test if there's no pressure information
		if metar.Pressure == 0 {
			continue
		}

		// Find the RMK section, BECMG section, and TEMPO section if they exist
		rmkIndex := -1
		becmgIndex := -1
		tempoIndex := -1
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
			}
			if part == "BECMG" {
				becmgIndex = i
			}
			if part == "TEMPO" {
				tempoIndex = i
			}
		}

		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}
		if becmgIndex != -1 && (rmkIndex == -1 || becmgIndex < rmkIndex) {
			endIndex = becmgIndex
		}
		if tempoIndex != -1 && tempoIndex < endIndex {
			endIndex = tempoIndex
		}

		found := false

		// Find the first pressure value in the main section, regardless of format
		var firstPressureIndex = -1
		var isPressureQ = false
		var isPressureA = false
		var expectedPressure float64

		for i := 2; i < endIndex; i++ {
			part := fields[i]

			// Check for Q format (hPa/millibars)
			if len(part) > 1 && part[0] == 'Q' {
				pressureStr := part[1:]
				pressureInt, err := strconv.Atoi(pressureStr)
				if err == nil && firstPressureIndex == -1 {
					firstPressureIndex = i
					isPressureQ = true
					expectedPressure = float64(pressureInt)
				}
			}

			// Check for A format (inHg)
			if pressureRegex.MatchString(part) {
				matches := pressureRegex.FindStringSubmatch(part)
				pressureStr := matches[1]
				pressureInt, err := strconv.Atoi(pressureStr)
				if err == nil && firstPressureIndex == -1 {
					firstPressureIndex = i
					isPressureA = true
					expectedPressure = float64(pressureInt) / 100.0
				}
			}
		}

		// Now validate based on the first pressure format found
		if firstPressureIndex != -1 {
			found = true
			if isPressureQ {
				assert.Equal(t, expectedPressure, metar.Pressure, line, "Q-format pressure parsed incorrectly")
				assert.Equal(t, "hPa", metar.PressureUnit, line, "Q-format pressure unit incorrectly set")
			} else if isPressureA {
				assert.Equal(t, expectedPressure, metar.Pressure, line, "A-format pressure parsed incorrectly")
				assert.Equal(t, "inHg", metar.PressureUnit, line, "A-format pressure unit incorrectly set")
			}
		}

		// If we still haven't found anything, we might have a pressure only in remarks,
		// but we don't look at that since we're only checking for pressures in the main section

		// Make sure we found and tested a pressure value if there's a non-remark pressure
		if metar.Pressure > 0 && !found {
			// Only assert failure for METARs known to have pressure in main section
			// This avoids spurious failures in test cases where pressure is only in the remarks
			mainSectionHasPressure := false
			for i := 2; i < endIndex; i++ {
				part := fields[i]
				if (len(part) > 1 && part[0] == 'Q') || pressureRegex.MatchString(part) {
					mainSectionHasPressure = true
					break
				}
			}
			if mainSectionHasPressure {
				assert.Fail(t, "expected to find pressure in main section but didn't", line)
			}
		}
	}
}

// TestDecodeMETAR_unhandledValues tests that all values in the METAR pre-remark section are handled
func TestDecodeMETAR_unhandledValues(t *testing.T) {
	t.Parallel()

	var unhandledValues []string

	for line, _ := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find the RMK section, BECMG section, and TEMPO section if they exist
		rmkIndex := -1
		becmgIndex := -1
		tempoIndex := -1
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break
			}
			if part == "BECMG" {
				becmgIndex = i
				break
			}
			if part == "TEMPO" {
				tempoIndex = i
				break
			}
		}

		// Determine the end of the pre-remark section (before RMK, BECMG, or TEMPO)
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}
		if becmgIndex != -1 && (becmgIndex < endIndex) {
			endIndex = becmgIndex
		}
		if tempoIndex != -1 && (tempoIndex < endIndex) {
			endIndex = tempoIndex
		}

		// Start at index 2 to skip station code and timestamp
		for i := 2; i < endIndex; i++ {
			part := fields[i]

			// Skip known handled patterns
			if windRegex.MatchString(part) || // Wind
				visRegexM.MatchString(part) || // Visibility
				isWeatherCode(part) || // Weather phenomena
				cloudRegex.MatchString(part) || // Clouds
				tempRegex.MatchString(part) || // Temperature/dewpoint
				(len(part) > 1 && part[0] == 'Q') || // Q-format pressure
				pressureRegex.MatchString(part) { // A-format pressure
				continue
			}

			// Skip CAVOK (ceiling and visibility OK) - special case
			if part == "CAVOK" {
				continue
			}

			// If we get here, we found an unhandled value
			unhandledValues = append(unhandledValues, part)
		}
	}

	// Check if we found any unhandled values
	if len(unhandledValues) > 0 {
		// Filter duplicates and sort
		slices.Sort(unhandledValues)
		unhandledValues = slices.Compact(unhandledValues)
		t.Errorf("Found unhandled values in METAR pre-remark section: %v", unhandledValues)
	}
}
