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
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		if fields[0] != metar.SiteInfo.Name {
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected station code: %s\nActual station code: %s\n\n",
				line, fields[0], metar.SiteInfo.Name))
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "STATION CODE FAILURES IN METAR PARSING\n"
		logContent += "=====================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "station_code_failures", logContent)

		t.Errorf("Found %d station code failures in METAR parsing. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_time(t *testing.T) {
	t.Parallel()
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		if fields[1] != "COR" {
			got := metar.Time.Format("021504") + "Z"
			if fields[1] != got {
				failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected time: %s\nActual time: %s\n\n",
					line, fields[1], got))
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "TIME PARSING FAILURES IN METAR\n"
		logContent += "============================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "time_parsing_failures", logContent)

		t.Errorf("Found %d time parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_remarks(t *testing.T) {
	t.Parallel()

	var unknownRemarks []string
	var failedMetars []string
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
			failedMetars = append(failedMetars, fmt.Sprintf("Raw METAR: %s\nUnknown remarks: %v\n\n",
				line, unknown))
		}
	}

	if failedRemarkCount > 0 {
		// Create log content
		slices.Sort(unknownRemarks)
		uniqueUnknownRemarks := slices.Compact(unknownRemarks)

		logContent := "UNKNOWN REMARK CODES IN METAR PARSING\n"
		logContent += "===================================\n\n"
		logContent += strings.Join(failedMetars, "")
		logContent += fmt.Sprintf("\nAll unique unknown remark codes: %v\n", uniqueUnknownRemarks)

		// Write to log file
		logFile := logTestFailures(t, "unknown_remark_failures", logContent)

		t.Errorf("Found %d METARs with unknown remark codes. See '%s' for details.",
			failedRemarkCount, logFile)
	}
}

func TestDecodeMETAR_visibility(t *testing.T) {
	t.Parallel()
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)

		// Find visibility value with SM suffix
		var expectedVisibility string
		for i := 1; i < len(fields); i++ {
			if strings.HasSuffix(fields[i], "SM") {
				// Check if this could be part of a spaced visibility like "1 1/2SM"
				if i > 1 && strings.Contains(fields[i], "/") &&
					!strings.HasPrefix(fields[i-1], "P") && !strings.HasPrefix(fields[i-1], "M") &&
					!strings.Contains(fields[i-1], "/") {
					// This is likely a split visibility like "1 1/2SM"
					expectedVisibility = fields[i-1] + " " + fields[i]
				} else {
					expectedVisibility = fields[i]
				}
				break
			}
		}

		if expectedVisibility != "" && expectedVisibility != metar.Visibility {
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected visibility: %s\nActual visibility: %s\n\n",
				line, expectedVisibility, metar.Visibility))
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "VISIBILITY PARSING FAILURES IN METAR\n"
		logContent += "=================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "visibility_parsing_failures", logContent)

		t.Errorf("Found %d visibility parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_wind(t *testing.T) {
	t.Parallel()
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		for _, field := range fields[1:] {
			if windRegex.MatchString(field) {
				expectedWind := parseWind(field)
				if expectedWind != metar.Wind {
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected wind: %+v\nActual wind: %+v\n\n",
						line, expectedWind, metar.Wind))
				}
				break
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "WIND PARSING FAILURES IN METAR\n"
		logContent += "============================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "wind_parsing_failures", logContent)

		t.Errorf("Found %d wind parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_weather(t *testing.T) {
	t.Parallel()
	var failures []string

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
			if part == "TEMPO" || part == "BECMG" {
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

		// Check if weather phenomena were parsed correctly
		if !assert.ElementsMatchf(t, expectedWeather, metar.Weather, "Raw METAR: %s", line) {
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected weather phenomena: %v\nActual weather phenomena: %v\n\n",
				line, expectedWeather, metar.Weather))
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "WEATHER PHENOMENA PARSING FAILURES IN METAR\n"
		logContent += "========================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "weather_parsing_failures", logContent)

		t.Errorf("Found %d weather phenomena parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_clouds(t *testing.T) {
	t.Parallel()
	var failures []string

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
			if part == "TEMPO" || part == "BECMG" {
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
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nWrong number of cloud layers - Expected: %d, Got: %d\nExpected clouds: %+v\nActual clouds: %+v\n\n",
				line, len(expectedClouds), len(metar.Clouds), expectedClouds, metar.Clouds))
			continue
		}

		// Check each cloud layer
		for i := range expectedClouds {
			if i < len(metar.Clouds) && expectedClouds[i] != metar.Clouds[i] {
				failures = append(failures, fmt.Sprintf("Raw METAR: %s\nCloud layer %d mismatch\nExpected: %+v\nActual: %+v\n\n",
					line, i, expectedClouds[i], metar.Clouds[i]))
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "CLOUD PARSING FAILURES IN METAR\n"
		logContent += "=============================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "cloud_parsing_failures", logContent)

		t.Errorf("Found %d cloud parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_temperature(t *testing.T) {
	t.Parallel()
	var failures []string

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
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nTemperature mismatch - Expected: %d, Got: %d\n",
						line, expectedTemp, metar.Temperature))
				}

				// Check if dew point is not nil before comparing
				if metar.DewPoint == nil {
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nDew point is nil but expected: %d\n\n",
						line, expectedDew))
				} else if expectedDew != *metar.DewPoint {
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nDew point mismatch - Expected: %d, Got: %d\n\n",
						line, expectedDew, *metar.DewPoint))
				}
				break
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "TEMPERATURE/DEW POINT PARSING FAILURES IN METAR\n"
		logContent += "============================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "temperature_parsing_failures", logContent)

		t.Errorf("Found %d temperature/dew point parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

func TestDecodeMETAR_pressure(t *testing.T) {
	t.Parallel()
	var failures []string

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
			if part == "TEMPO" || part == "BECMG" {
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
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nQ-format pressure mismatch - Expected: %.2f hPa, Got: %.2f %s\n\n",
						line, expectedPressure, metar.Pressure, metar.PressureUnit))
				}
			} else if isPressureA {
				if expectedPressure != metar.Pressure || metar.PressureUnit != "inHg" {
					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nA-format pressure mismatch - Expected: %.2f inHg, Got: %.2f %s\n\n",
						line, expectedPressure, metar.Pressure, metar.PressureUnit))
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
				failures = append(failures, fmt.Sprintf("Raw METAR: %s\nExpected to find pressure in main section but didn't. Decoded pressure: %.2f %s\n\n",
					line, metar.Pressure, metar.PressureUnit))
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "PRESSURE PARSING FAILURES IN METAR\n"
		logContent += "================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "pressure_parsing_failures", logContent)

		t.Errorf("Found %d pressure parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}

// TestDecodeMETAR_unhandledValues tests that all values in the METAR pre-remark section are handled
func TestDecodeMETAR_unhandledValues(t *testing.T) {
	t.Parallel()

	// Map to store METARs with their unhandled values
	unhandledByMetar := make(map[string][]string)
	var allUnhandledValues []string

	for line, _ := range decodeMETARList(t) {
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
			if part == "TEMPO" || part == "BECMG" {
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

		// Track unhandled values for this specific METAR
		var metarUnhandledValues []string

		// Start at index 2 to skip station code and timestamp
		for i := 2; i < endIndex; i++ {
			part := fields[i]

			// Skip known handled patterns
			if windRegex.MatchString(part) || // Wind in KT format
				windRegexMPS.MatchString(part) || // Wind in MPS format
				eWindRegex.MatchString(part) || // Wind with E prefix
				windVarRegex.MatchString(part) || // Wind direction variation
				visRegexM.MatchString(part) || // Visibility in SM format
				visRegexNum.MatchString(part) || // Visibility in meters (4-digit number)
				visRegexDir.MatchString(part) || // Visibility with direction
				ndvRegex.MatchString(part) || // Visibility with No Directional Variation
				isWeatherCode(part) || // Weather phenomena
				cloudRegex.MatchString(part) || // Clouds
				extCloudRegex.MatchString(part) || // Extended cloud format
				vvRegex.MatchString(part) || // Vertical visibility
				specialRegex.MatchString(part) || // Special codes
				tempRegex.MatchString(part) || // Temperature/dewpoint
				tempOnlyRegex.MatchString(part) || // Temperature only format (M01/)
				(len(part) > 1 && part[0] == 'Q') || // Q-format pressure
				pressureRegex.MatchString(part) || // A-format pressure
				cavokRegex.MatchString(part) || // CAVOK
				rvrRegex.MatchString(part) || // Basic Runway Visual Range
				runwayCondRegex.MatchString(part) || // Enhanced runway condition
				runwayClearedRegex.MatchString(part) { // Cleared runway condition
				continue
			}

			// Skip CAVOK (ceiling and visibility OK)
			if part == "CAVOK" {
				continue
			}

			// Skip the integer part of a spaced visibility value like "1 1/2SM"
			if i+1 < endIndex && strings.HasSuffix(fields[i+1], "SM") &&
				!strings.HasPrefix(part, "P") && !strings.HasPrefix(part, "M") &&
				!strings.Contains(part, "/") {
				continue
			}

			// If we get here, we found an unhandled value
			metarUnhandledValues = append(metarUnhandledValues, part)
			allUnhandledValues = append(allUnhandledValues, part)
		}

		// If we found unhandled values for this METAR, store them
		if len(metarUnhandledValues) > 0 {
			unhandledByMetar[line] = metarUnhandledValues
		}
	}

	// Check if we found any unhandled values
	if len(allUnhandledValues) > 0 {
		// Filter duplicates and sort for the overall list
		slices.Sort(allUnhandledValues)
		allUnhandledValues = slices.Compact(allUnhandledValues)

		// Create log content
		logContent := "UNHANDLED VALUES IN METAR PRE-REMARK SECTION\n"
		logContent += "=======================================\n\n"

		// Write each problematic METAR and its unhandled values to the log
		for metar, unhandledValues := range unhandledByMetar {
			logContent += fmt.Sprintf("Raw METAR: %s\nUnhandled values: %v\n\n", metar, unhandledValues)
		}

		// Write the overall list of unique unhandled values
		logContent += fmt.Sprintf("All unique unhandled values: %v\n", allUnhandledValues)

		// Write to log file
		logFile := logTestFailures(t, "unhandled_metar_values", logContent)

		// Report to the test output
		t.Errorf("Found %d unhandled values in METAR pre-remark section. See '%s' for details.",
			len(allUnhandledValues), logFile)
	}
}
func TestDecodeMETAR_runwayConditions(t *testing.T) {
	t.Parallel()
	var failures []string

	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		var expectedRunwayConditions []RunwayCondition

		// Find sections to know where to stop
		rmkIndex := -1
		sectionIndices := []int{}

		// Find all TEMPO, BECMG, and RMK sections
		for i, part := range fields {
			if part == "RMK" {
				rmkIndex = i
				break // RMK always ends the main section
			}
			if part == "TEMPO" || part == "BECMG" {
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

		// Collect runway condition data from original METAR
		for i := 2; i < endIndex; i++ {
			if runwayCondRegex.MatchString(fields[i]) || runwayClearedRegex.MatchString(fields[i]) {
				expectedRunwayConditions = append(expectedRunwayConditions, parseRunwayCondition(fields[i]))
			}
		}

		// Check number of runway conditions
		if len(expectedRunwayConditions) != len(metar.RunwayConditions) {
			failures = append(failures, fmt.Sprintf("Raw METAR: %s\nWrong number of runway conditions - Expected: %d, Got: %d\nExpected: %+v\nActual: %+v\n\n",
				line, len(expectedRunwayConditions), len(metar.RunwayConditions), expectedRunwayConditions, metar.RunwayConditions))
			continue
		}

		// Check each runway condition
		for i := range expectedRunwayConditions {
			if i < len(metar.RunwayConditions) {
				expected := expectedRunwayConditions[i]
				actual := metar.RunwayConditions[i]

				// Compare each field individually for better error reporting
				if expected.Runway != actual.Runway ||
					expected.Visibility != actual.Visibility ||
					expected.VisMin != actual.VisMin ||
					expected.VisMax != actual.VisMax ||
					expected.Trend != actual.Trend ||
					expected.Unit != actual.Unit ||
					expected.Prefix != actual.Prefix ||
					expected.Cleared != actual.Cleared ||
					expected.ClearedTime != actual.ClearedTime {

					failures = append(failures, fmt.Sprintf("Raw METAR: %s\nRunway condition %d mismatch\nExpected: %+v\nActual: %+v\n\n",
						line, i, expected, actual))

					// Add detailed field differences for better debugging
					if expected.Runway != actual.Runway {
						failures = append(failures, fmt.Sprintf("  - Runway mismatch: expected '%s', got '%s'\n", expected.Runway, actual.Runway))
					}
					if expected.Visibility != actual.Visibility {
						failures = append(failures, fmt.Sprintf("  - Visibility mismatch: expected %d, got %d\n", expected.Visibility, actual.Visibility))
					}
					if expected.VisMin != actual.VisMin {
						failures = append(failures, fmt.Sprintf("  - VisMin mismatch: expected %d, got %d\n", expected.VisMin, actual.VisMin))
					}
					if expected.VisMax != actual.VisMax {
						failures = append(failures, fmt.Sprintf("  - VisMax mismatch: expected %d, got %d\n", expected.VisMax, actual.VisMax))
					}
					if expected.Trend != actual.Trend {
						failures = append(failures, fmt.Sprintf("  - Trend mismatch: expected '%s', got '%s'\n", expected.Trend, actual.Trend))
					}
					if expected.Unit != actual.Unit {
						failures = append(failures, fmt.Sprintf("  - Unit mismatch: expected '%s', got '%s'\n", expected.Unit, actual.Unit))
					}
					if expected.Prefix != actual.Prefix {
						failures = append(failures, fmt.Sprintf("  - Prefix mismatch: expected '%s', got '%s'\n", expected.Prefix, actual.Prefix))
					}
					if expected.Cleared != actual.Cleared {
						failures = append(failures, fmt.Sprintf("  - Cleared mismatch: expected %v, got %v\n", expected.Cleared, actual.Cleared))
					}
					if expected.ClearedTime != actual.ClearedTime {
						failures = append(failures, fmt.Sprintf("  - ClearedTime mismatch: expected %d, got %d\n", expected.ClearedTime, actual.ClearedTime))
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		// Create log content
		logContent := "RUNWAY CONDITION PARSING FAILURES IN METAR\n"
		logContent += "=======================================\n\n"
		logContent += strings.Join(failures, "")

		// Write to log file
		logFile := logTestFailures(t, "runway_condition_parsing_failures", logContent)

		t.Errorf("Found %d runway condition parsing failures in METAR. See '%s' for details.",
			len(failures), logFile)
	}
}
