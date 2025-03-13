package main

import (
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rmitchellscott/WxCraft/testdata"
	"github.com/stretchr/testify/assert"
)

// createTestLogDirectory creates a directory for test logs if it doesn't exist
func createTestLogDirectory(t *testing.T) string {
	// Create logs directory
	logDir := "test-logs"
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test log directory: %v", err)
	}
	return logDir
}

// logTestFailures writes the failure information to a log file
func logTestFailures(t *testing.T, testName string, content string) string {
	logDir := createTestLogDirectory(t)

	// Create log filename with timestamp to avoid overwrites
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s_%s.log", testName, timestamp)
	logFilePath := filepath.Join(logDir, logFileName)

	err := os.WriteFile(logFilePath, []byte(content), 0644)
	if err != nil {
		t.Errorf("Failed to write to log file: %v", err)
		return ""
	}

	return logFilePath
}

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
		if fields[0] != metar.SiteInfo.Name {
			t.Run(line, func(t *testing.T) {
				t.Errorf("Raw METAR: %s\nExpected station code: %s\nActual station code: %s\n\n",
					line, fields[0], metar.SiteInfo.Name)
			})
		}
	}
}

func TestDecodeMETAR_time(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		if fields[1] != "COR" {
			got := metar.Time.Format("021504") + "Z"
			if fields[1] != got {
				t.Run(line, func(t *testing.T) {
					t.Errorf("Raw METAR: %s\nExpected time: %s\nActual time: %s\n\n",
						line, fields[1], got)
				})
			}
		}
	}
}

func TestDecodeMETAR_remarks(t *testing.T) {
	t.Parallel()

	var unknownValues []string
	var failedValueCount int

	for line, metar := range decodeMETARList(t) {
		unknown := make([]string, 0, len(metar.Remarks))
		for _, rmk := range metar.Remarks {
			if rmk.Description == "unknown remark code" {
				unknown = append(unknown, rmk.Raw)
				unknownValues = append(unknownValues, rmk.Raw)
			}
		}

		if len(unknown) != 0 {
			failedValueCount++
			t.Run(line, func(t *testing.T) {
				t.Errorf("Unknown remarks:\nMETAR   = %s\nRemarks = %v", line, unknown)
			})
		}
	}

	t.Run("unknown remark count", func(t *testing.T) {
		slices.Sort(unknownValues)
		unknownValues = slices.Compact(unknownValues)
		assert.Empty(t, len(unknownValues))
	})

	t.Run("metars with failed remarks", func(t *testing.T) {
		assert.Zero(t, failedValueCount)
	})
}
func TestDecodeMETAR_weatherCode(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" || part == "INTER" {
				sectionIndices = append(sectionIndices, i)
			}
		}

		// Find the first section marker
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Find the earliest TEMPO or BECMG section
		for _, idx := range sectionIndices {
			if idx < endIndex {
				endIndex = idx
			}
		}

		// Collect weather codes from original METAR
		var expectedWeatherCodes []string
		for i := 2; i < endIndex; i++ {
			if isWeatherCode(fields[i]) {
				expectedWeatherCodes = append(expectedWeatherCodes, fields[i])
			}
		}
		// Filter out WS which is handled separately as wind shear
		expectedWeatherCodes = slices.DeleteFunc(expectedWeatherCodes, func(s string) bool {
			return s == "WS"
		})

		// Compare with decoded weather codes
		if !slices.Equal(expectedWeatherCodes, metar.Weather) {
			t.Run(line, func(t *testing.T) {
				t.Errorf("Raw METAR: %s\nExpected weather codes: %v\nActual weather codes: %v\n\n",
					line, expectedWeatherCodes, metar.Weather)
			})
		}
	}
}
func TestDecodeMETAR_visibility(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find visibility value with SM suffix
		var expectedVisibility string
		for i := 1; i < len(fields); i++ {
			if strings.HasSuffix(fields[i], "SM") {
				// Check if this could be part of a spaced visibility like "1 1/2SM"
				if i > 1 && strings.Contains(fields[i], "/") &&
					!strings.HasPrefix(fields[i-1], "P") && !strings.HasPrefix(fields[i-1], "M") &&
					!strings.Contains(fields[i-1], "/") && len(fields[i-1]) == 1 {
					// This is likely a split visibility like "1 1/2SM"
					expectedVisibility = fields[i-1] + " " + fields[i]
				} else {
					expectedVisibility = fields[i]
				}
				break
			}
		}

		if expectedVisibility != "" && expectedVisibility != metar.Visibility {
			t.Run(line, func(t *testing.T) {
				t.Errorf("Raw METAR: %s\nExpected visibility: %s\nActual visibility: %s\n\n",
					line, expectedVisibility, metar.Visibility)
			})
		}
	}
}
func TestDecodeMETAR_verticalVisibility(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" || part == "INTER" {
				sectionIndices = append(sectionIndices, i)
			}
		}

		// Find the first section marker
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Find the earliest TEMPO or BECMG section
		for _, idx := range sectionIndices {
			if idx < endIndex {
				endIndex = idx
			}
		}

		// Find vertical visibility in the original METAR
		var expectedVertVis int
		for i := 2; i < endIndex; i++ {
			if isVerticalVisibility(fields[i]) {
				matches := vvRegex.FindStringSubmatch(fields[i])
				if len(matches) > 1 {
					expectedVertVis, _ = strconv.Atoi(matches[1])
					break
				}
			}
		}

		// Only test if vertical visibility was found in the original METAR
		if expectedVertVis > 0 || metar.VertVis > 0 {
			if expectedVertVis != metar.VertVis {
				t.Run(line, func(t *testing.T) {
					t.Errorf("Raw METAR: %s\nExpected vertical visibility: %d00ft\nActual vertical visibility: %d00ft\n\n",
						line, expectedVertVis, metar.VertVis)
				})
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
				if expectedWind != metar.Wind {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nExpected wind: %+v\nActual wind: %+v\n\n",
							line, expectedWind, metar.Wind)
					})
				}
				break
			}
		}
	}
}

func TestDecodeMETAR_windshear(t *testing.T) {
	t.Parallel()
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedWindShear []WindShear

		// Find wind shear in the original METAR
		// Skip station codes and remark sections
		rmkIndex := -1
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break
			}
		}
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Only look for wind shear in the main section (not in remarks)
		for i := 1; i < endIndex; i++ {
			// Handle "WS ALL RWY" pattern
			if i+2 < endIndex && fields[i] == "WS" && fields[i+1] == "ALL" && fields[i+2] == "RWY" {
				expectedWindShear = append(expectedWindShear, WindShear{
					Type:  "RWY",
					Phase: "ALL",
					Raw:   "WS ALL RWY",
				})
				i += 2 // Skip the next two tokens
			} else if i+1 < endIndex && fields[i] == "WS" && strings.HasPrefix(fields[i+1], "R") && len(fields[i+1]) > 1 {
				// Handle "WS R##" pattern
				expectedWindShear = append(expectedWindShear, WindShear{
					Type:   "RWY",
					Runway: fields[i+1][1:], // Remove the 'R' prefix
					Raw:    fields[i] + " " + fields[i+1],
				})
				i++ // Skip the next token
			} else if strings.HasPrefix(fields[i], "WS") && fields[i] != "WS" &&
				!strings.HasPrefix(fields[i], "WSSS") &&
				!strings.HasPrefix(fields[i], "WSSL") &&
				!strings.HasPrefix(fields[i], "WSAP") &&
				!strings.HasPrefix(fields[i], "WSHFT") {
				// Single-token wind shear format
				// Skip station codes that start with WS
				// Skip WSHFT which is a wind shift in remarks
				expectedWindShear = append(expectedWindShear, parseWindShear(fields[i]))
			}
		}

		// Skip if no wind shear in the raw METAR
		if len(expectedWindShear) == 0 && len(metar.WindShear) == 0 {
			continue
		}

		// Check if wind shear was parsed correctly
		if len(expectedWindShear) != len(metar.WindShear) {
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nWrong number of wind shear entries - Expected: %d, Got: %d\nExpected: %+v\nActual: %+v\n\n",
				line, len(expectedWindShear), len(metar.WindShear), expectedWindShear, metar.WindShear))
			continue
		}

		// Compare each wind shear entry
		for i, expected := range expectedWindShear {
			actual := metar.WindShear[i]

			// Compare Type, Runway and Phase fields which are most relevant
			if expected.Type != actual.Type ||
				expected.Runway != actual.Runway ||
				expected.Phase != actual.Phase {

				failures = append(failures, fmt.Sprintf("Raw METAR: %s\nWind shear entry mismatch\nExpected: %+v\nActual: %+v\n\n",
					line, expected, actual))
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "WIND SHEAR PARSING FAILURES IN METAR\n"
		logContent += "==================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "wind_shear_parsing_failures", logContent)

		t.Errorf("Found %d wind shear parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_weather(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedWeather []string

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" || part == "INTER" {
				sectionIndices = append(sectionIndices, i)
			}
		}

		// Find the first section marker
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Find the earliest TEMPO or BECMG section
		for _, idx := range sectionIndices {
			if idx < endIndex {
				endIndex = idx
			}
		}

		// Collect weather phenomena from original METAR
		for i := 2; i < endIndex; i++ {
			if isWeatherCode(fields[i]) {
				expectedWeather = append(expectedWeather, fields[i])
			}
		}
		expectedWeather = slices.DeleteFunc(expectedWeather, func(s string) bool {
			return s == "WS"
		})

		// Check if weather phenomena were parsed correctly
		if !slices.Equal(expectedWeather, metar.Weather) {
			t.Run(line, func(t *testing.T) {
				t.Errorf("Raw METAR: %s\nExpected weather phenomena: %v\nActual weather phenomena: %v\n\n",
					line, expectedWeather, metar.Weather)
			})
		}
	}
}
func TestDecodeMETAR_clouds(t *testing.T) {
	t.Parallel()

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedClouds []Cloud

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" || part == "INTER" {
				sectionIndices = append(sectionIndices, i)
			}
		}

		// Find the first section marker
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Find the earliest TEMPO or BECMG section
		for _, idx := range sectionIndices {
			if idx < endIndex {
				endIndex = idx
			}
		}

		// Collect cloud data from original METAR
		for i := 2; i < endIndex; i++ {
			if cloudRegex.MatchString(fields[i]) {
				expectedClouds = append(expectedClouds, parseCloud(fields[i]))
			}
		}

		// Check number of cloud layers
		if len(expectedClouds) != len(metar.Clouds) {
			t.Run(line, func(t *testing.T) {
				t.Errorf("Raw METAR: %s\nWrong number of cloud layers - Expected: %d, Got: %d\nExpected clouds: %+v\nActual clouds: %+v\n\n",
					line, len(expectedClouds), len(metar.Clouds), expectedClouds, metar.Clouds)
			})
			continue
		}

		// Check each cloud layer
		for i := range expectedClouds {
			if i < len(metar.Clouds) && expectedClouds[i] != metar.Clouds[i] {
				t.Run(line, func(t *testing.T) {
					t.Errorf("Raw METAR: %s\nCloud layer %d mismatch\nExpected: %+v\nActual: %+v\n\n",
						line, i, expectedClouds[i], metar.Clouds[i])
				})
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

				if expectedTemp != metar.Temperature {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nTemperature mismatch - Expected: %d, Got: %d\n",
							line, expectedTemp, metar.Temperature)
					})
				}

				// Check if dew point is not nil before comparing
				if metar.DewPoint == nil {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nDew point is nil but expected: %d\n\n",
							line, expectedDew)
					})
				} else if expectedDew != *metar.DewPoint {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nDew point mismatch - Expected: %d, Got: %d\n\n",
							line, expectedDew, *metar.DewPoint)
					})
				}
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

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" || part == "INTER" {
				sectionIndices = append(sectionIndices, i)
			}
		}

		// Find the first section marker
		endIndex := len(fields)
		if rmkIndex != -1 {
			endIndex = rmkIndex
		}

		// Find the earliest TEMPO or BECMG section
		for _, idx := range sectionIndices {
			if idx < endIndex {
				endIndex = idx
			}
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
				if expectedPressure != metar.Pressure || metar.PressureUnit != "hPa" {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nQ-format pressure mismatch - Expected: %.2f hPa, Got: %.2f %s\n\n",
							line, expectedPressure, metar.Pressure, metar.PressureUnit)
					})
				}
			} else if isPressureA {
				if expectedPressure != metar.Pressure || metar.PressureUnit != "inHg" {
					t.Run(line, func(t *testing.T) {
						t.Errorf("Raw METAR: %s\nA-format pressure mismatch - Expected: %.2f inHg, Got: %.2f %s\n\n",
							line, expectedPressure, metar.Pressure, metar.PressureUnit)
					})
				}
			}
		}

		// Make sure we found and tested a pressure value if there's a non-remark pressure
		if metar.Pressure > 0 && !found {
			// Only check for METARs known to have pressure in main section
			mainSectionHasPressure := false
			for i := 2; i < endIndex; i++ {
				part := fields[i]
				if (len(part) > 1 && part[0] == 'Q') || pressureRegex.MatchString(part) {
					mainSectionHasPressure = true
					break
				}
			}
			if mainSectionHasPressure {
				t.Run(line, func(t *testing.T) {
					t.Errorf("Raw METAR: %s\nExpected to find pressure in main section but didn't. Decoded pressure: %.2f %s\n\n",
						line, metar.Pressure, metar.PressureUnit)
				})
			}
		}
	}
}

// TestDecodeMETAR_unhandledValues tests that all values in the METAR pre-remark section are handled
func TestDecodeMETAR_unhandledValues(t *testing.T) {
	t.Parallel()

	var failedValueCount int

	for line, metar := range decodeMETARList(t) {

		if len(metar.Unhandled) != 0 {
			failedValueCount++
			t.Run(line, func(t *testing.T) {
				t.Errorf("Unknown value:\nMETAR   = %s\nValue = %v", line, metar.Unhandled)
			})
		}
	}

	t.Run("00 metars with failed values", func(t *testing.T) {
		assert.Zero(t, failedValueCount)
	})
}
