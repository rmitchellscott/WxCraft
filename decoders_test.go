package main

import (
	"cmp"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rmitchellscott/WxCraft/testdata"
)

// Every corpus report was issued on day 08, so any reference shortly after it decodes to the same month.
var corpusReference = time.Date(2025, time.March, 10, 0, 0, 0, 0, time.UTC)

// coverageThresholds caps each corpus coverage count. A count above its cap fails the test;
// a count below it is logged so the cap can be lowered once a fix lands.
var coverageThresholds = map[string]int{
	"metar.station_mismatch":             0,
	"metar.time_mismatch":                1,
	"metar.body_unhandled":               162,
	"metar.body_unrepresented":           32,
	"metar.weather_not_weather":          0,
	"metar.weather_missed":               0,
	"metar.wind_mismatch":                0,
	"metar.visibility_mismatch":          0,
	"metar.vertical_visibility_mismatch": 0,
	"metar.cloud_mismatch":               0,
	"metar.temperature_mismatch":         0,
	"metar.pressure_mismatch":            0,
	"metar.remark_unknown":               910,
	"taf.station_mismatch":               0,
	"taf.period_count_mismatch":          83,
	"taf.weather_not_weather":            0,
	"taf.unrepresented":                  9201,
}

var (
	testWeatherGroupRegex = regexp.MustCompile(`^(?:RE)?(?:[+-]?(?:VC)?(?:MI|PR|BC|DR|BL|SH|TS|FZ)?(?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS)*|//)$`)
	testWindRegex         = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(?:G(\d{2,3}))?(KT|MPS|KMH)$`)
	testCloudRegex        = regexp.MustCompile(`^(FEW|SCT|BKN|OVC|///)(\d{3}|///)?(CB|TCU|///)?$|^(SKC|CLR)$|^(FEW|SCT|BKN|OVC)(CB|TCU)(\d{3})$`)
	testVertVisRegex      = regexp.MustCompile(`^VV(\d{3})$`)
	testTempRegex         = regexp.MustCompile(`^(M)?(\d{1,2})/(M)?(\d{1,2})$`)
	testPressureRegex     = regexp.MustCompile(`^([AQ])(\d{4})$`)
	testIssueTimeRegex    = regexp.MustCompile(`^\d{6}Z$`)
	testPeriodRegex       = regexp.MustCompile(`^\d{4}/\d{4}$`)
	testFromRegex         = regexp.MustCompile(`^FM\d{6}$`)
	testProbRegex         = regexp.MustCompile(`^PROB\d{2}$`)
	testDigitRegex        = regexp.MustCompile(`\d`)
	testGluedPressure     = regexp.MustCompile(`^([AQ]\d{4})(NOSIG|TEMPO|BECMG|RMK)$`)
)

var testHeaderKeywords = []string{"TAF", "METAR", "SPECI", "AMD", "COR", "RTD"}

type coverageReport struct {
	counts map[string]int
	shapes map[string]map[string]int
}

func newCoverageReport() *coverageReport {
	return &coverageReport{counts: map[string]int{}, shapes: map[string]map[string]int{}}
}

func (c *coverageReport) add(metric, key string) {
	c.counts[metric]++
	if c.shapes[metric] == nil {
		c.shapes[metric] = map[string]int{}
	}
	c.shapes[metric][key]++
}

// check logs every metric with its most common shapes and fails only on counts above threshold
func (c *coverageReport) check(t *testing.T, metrics []string) {
	t.Helper()
	for _, metric := range metrics {
		count := c.counts[metric]
		threshold, ok := coverageThresholds[metric]
		if !ok {
			t.Errorf("%s: no coverage threshold set (count %d)", metric, count)
			continue
		}

		t.Logf("%-34s %6d  (threshold %d, %d distinct)", metric, count, threshold, len(c.shapes[metric]))
		for _, shape := range topShapes(c.shapes[metric], 15) {
			t.Logf("    %6d  %s", c.shapes[metric][shape], shape)
		}

		switch {
		case count > threshold:
			t.Errorf("%s regressed: %d > threshold %d", metric, count, threshold)
		case count < threshold:
			t.Logf("%s improved: lower its threshold from %d to %d", metric, threshold, count)
		}
	}
}

func topShapes(shapes map[string]int, n int) []string {
	keys := make([]string, 0, len(shapes))
	for key := range shapes {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(cmp.Compare(shapes[b], shapes[a]), cmp.Compare(a, b))
	})
	return keys[:min(n, len(keys))]
}

func tokenShape(token string) string {
	return testDigitRegex.ReplaceAllString(token, "9")
}

func isTestWeatherGroup(token string) bool {
	switch token {
	case "", "+", "-", "VC", "RE", "-VC", "+VC":
		return false
	}
	return testWeatherGroupRegex.MatchString(token)
}

// headerLength returns how many leading fields are report keywords, station, issue time and validity
func headerLength(fields []string) int {
	i := 0
	for i < len(fields) && slices.Contains(testHeaderKeywords, fields[i]) {
		i++
	}
	i++
	if i < len(fields) && testIssueTimeRegex.MatchString(fields[i]) {
		i++
	}
	if i < len(fields) && testPeriodRegex.MatchString(fields[i]) {
		i++
	}
	return min(i, len(fields))
}

func expectedStation(fields []string) string {
	for _, field := range fields {
		if !slices.Contains(testHeaderKeywords, field) {
			return field
		}
	}
	return ""
}

func indexOrLen(fields []string, stop ...string) int {
	for i, field := range fields {
		if slices.Contains(stop, field) {
			return i
		}
	}
	return len(fields)
}

// representedTokens collects the strings a decoded report carries in its exported fields, plus
// wind, cloud, vertical visibility, temperature and pressure rendered back to their raw form.
// A raw token missing from the set was dropped by the decoder. The report's own Raw text,
// Unhandled and Remarks are left out so they cannot vouch for a token.
func representedTokens(decoded any) map[string]bool {
	set := map[string]bool{}
	collectStrings(reflect.ValueOf(decoded), set)
	return set
}

var (
	windType  = reflect.TypeFor[Wind]()
	cloudType = reflect.TypeFor[Cloud]()
)

func collectStrings(v reflect.Value, set map[string]bool) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			collectStrings(v.Elem(), set)
		}
	case reflect.String:
		set[v.String()] = true
		for _, part := range strings.Fields(v.String()) {
			set[part] = true
		}
	case reflect.Slice:
		for i := range v.Len() {
			collectStrings(v.Index(i), set)
		}
	case reflect.Struct:
		switch v.Type() {
		case windType:
			wind := v.Interface().(Wind)
			if wind.Speed != nil {
				gust := ""
				if wind.Gust > 0 {
					gust = fmt.Sprintf("G%02d", wind.Gust)
				}
				set[fmt.Sprintf("%s%02d%s%s", wind.Direction, *wind.Speed, gust, wind.Unit)] = true
			} else if wind.Direction == "///" {
				set["/////"+wind.Unit] = true
			}
		case cloudType:
			cloud := v.Interface().(Cloud)
			set[cloud.Coverage] = true
			if cloud.HeightMissing {
				set[cloud.Coverage+cloud.Type] = true
				set[cloud.Coverage+"///"+cloud.Type] = true
			} else {
				set[fmt.Sprintf("%s%03d%s", cloud.Coverage, cloud.Height/100, cloud.Type)] = true
			}
		}
		for i := range v.NumField() {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			switch field.Name {
			case "Unhandled", "Remarks":
				continue
			case "Raw":
				if v.Type() == reflect.TypeFor[WeatherData]() || v.Type() == reflect.TypeFor[Forecast]() {
					continue
				}
			}
			collectStrings(v.Field(i), set)
		}
	}
}

func addMETARRenderings(metar METAR, set map[string]bool) {
	if metar.VertVis != nil {
		set[fmt.Sprintf("VV%03d", *metar.VertVis)] = true
	}
	if metar.VertVisMissing {
		set["VV///"] = true
	}
	if metar.Temperature == nil && metar.DewPoint == nil {
		set["/////"] = true
	}
	if metar.Temperature != nil && metar.DewPoint != nil {
		for _, temperature := range signedTemps(*metar.Temperature) {
			for _, dewPoint := range signedTemps(*metar.DewPoint) {
				set[temperature+"/"+dewPoint] = true
			}
		}
	} else if metar.Temperature != nil {
		for _, temperature := range signedTemps(*metar.Temperature) {
			set[temperature+"/"] = true
			set[temperature+"///"] = true
		}
	}
	if metar.StationPressure > 0 {
		qfe := fmt.Sprintf("%.1f", metar.StationPressure)
		set["QFE"] = true
		set[qfe] = true
		set["QFE"+qfe] = true
		set[strings.Replace(qfe, ".", "/", 1)] = true
	}
	for _, trend := range metar.Trends {
		addForecastRenderings(trend, set)
		for prefix, at := range map[string]time.Time{"FM": trend.From, "TL": trend.To, "AT": trend.At} {
			if !at.IsZero() {
				set[prefix+at.Format("1504")] = true
			}
		}
	}
	for _, pressure := range []PressureReading{{metar.Pressure, metar.PressureUnit}, metar.SecondPressure} {
		switch {
		case pressure.Value == 0 && pressure.Unit == "inHg":
			set["A////"] = true
		case pressure.Value == 0 && pressure.Unit == "hPa":
			set["Q////"] = true
		}
		switch pressure.Unit {
		case "inHg":
			set[fmt.Sprintf("A%04d", int(math.Round(pressure.Value*100)))] = true
		case "hPa":
			set[fmt.Sprintf("Q%04d", int(pressure.Value))] = true
		}
	}
}

func addForecastRenderings(period Forecast, set map[string]bool) {
	if period.VertVis != nil {
		set[fmt.Sprintf("VV%03d", *period.VertVis)] = true
	}
	if period.NSC {
		set["NSC"] = true
	}
	if period.NSW {
		set["NSW"] = true
	}
	if period.PressureUnit == "inHg" {
		set[fmt.Sprintf("QNH%04dINS", int(math.Round(period.Pressure*100)))] = true
	}
}

// signedTemps renders whole Celsius as METAR text; 0 has two forms because M00 means just below
// zero, and a few stations drop the leading zero, as in 24/5
func signedTemps(celsius int) []string {
	switch {
	case celsius < 0:
		return []string{fmt.Sprintf("M%02d", -celsius), fmt.Sprintf("M%d", -celsius)}
	case celsius == 0:
		return []string{"00", "M00", "0", "M0"}
	}
	return []string{fmt.Sprintf("%02d", celsius), fmt.Sprintf("%d", celsius)}
}

var metarCoverageMetrics = []string{
	"metar.station_mismatch",
	"metar.time_mismatch",
	"metar.body_unhandled",
	"metar.body_unrepresented",
	"metar.weather_not_weather",
	"metar.weather_missed",
	"metar.wind_mismatch",
	"metar.visibility_mismatch",
	"metar.vertical_visibility_mismatch",
	"metar.cloud_mismatch",
	"metar.temperature_mismatch",
	"metar.pressure_mismatch",
	"metar.remark_unknown",
}

// TestMETARCorpusCoverage decodes the METAR corpus and checks each field against an
// expectation built from test-local regexes, never from the decoder's own helpers.
func TestMETARCorpusCoverage(t *testing.T) {
	t.Parallel()

	report := newCoverageReport()
	scanner := testdata.METAR(t)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		checkMETAR(line, DecodeMETAR(line, corpusReference), report)
	}

	report.check(t, metarCoverageMetrics)
}

// splitGluedPressure separates a pressure group that runs into the next keyword, as in "Q1006TEMPO",
// the way the decoder reads it
func splitGluedPressure(fields []string) []string {
	var split []string
	for _, field := range fields {
		if m := testGluedPressure.FindStringSubmatch(field); m != nil {
			split = append(split, m[1], m[2])
			continue
		}
		split = append(split, field)
	}
	return split
}

func checkMETAR(line string, metar METAR, report *coverageReport) {
	fields := splitGluedPressure(strings.Fields(line))
	start := headerLength(fields)
	station := expectedStation(fields)
	bodyEnd := indexOrLen(fields, "RMK")
	trendStart := indexOrLen(fields[:bodyEnd], "TEMPO", "BECMG", "INTER")
	body := fields[start:trendStart]

	if metar.Station != station {
		report.add("metar.station_mismatch", metar.Station)
	}

	for _, field := range fields[:min(start+2, len(fields))] {
		if testIssueTimeRegex.MatchString(field) {
			if got := metar.Time.Format("021504") + "Z"; got != field {
				report.add("metar.time_mismatch", tokenShape(field))
			}
			break
		}
	}

	for _, token := range metar.Unhandled {
		report.add("metar.body_unhandled", tokenShape(token))
	}

	represented := representedTokens(metar)
	addMETARRenderings(metar, represented)
	for _, token := range fields[start:bodyEnd] {
		if !represented[token] && !slices.Contains(metar.Unhandled, token) {
			report.add("metar.body_unrepresented", tokenShape(token))
		}
	}

	for _, weather := range metar.Weather {
		if !isTestWeatherGroup(weather) {
			report.add("metar.weather_not_weather", weather)
		}
	}
	for _, token := range body {
		if isTestWeatherGroup(token) && !slices.Contains(metar.Weather, token) {
			report.add("metar.weather_missed", token)
		}
	}

	for _, token := range body {
		if m := testWindRegex.FindStringSubmatch(token); m != nil {
			speed, _ := strconv.Atoi(m[2])
			gust, _ := strconv.Atoi(m[3])
			wind := metar.Wind
			if wind.Direction != m[1] || wind.Speed == nil || *wind.Speed != speed || wind.Gust != gust || wind.Unit != m[4] {
				report.add("metar.wind_mismatch", tokenShape(token))
			}
			break
		}
	}

	if visibility := expectedSMVisibility(fields[start:bodyEnd]); visibility != "" && visibility != metar.Visibility {
		report.add("metar.visibility_mismatch", tokenShape(visibility))
	}

	for _, token := range body {
		if m := testVertVisRegex.FindStringSubmatch(token); m != nil {
			if height, _ := strconv.Atoi(m[1]); metar.VertVis == nil || height != *metar.VertVis {
				report.add("metar.vertical_visibility_mismatch", tokenShape(token))
			}
			break
		}
	}

	var clouds []Cloud
	for _, token := range body {
		if m := testCloudRegex.FindStringSubmatch(token); m != nil {
			if m[4] != "" {
				clouds = append(clouds, Cloud{Coverage: m[4]})
				continue
			}
			if m[5] != "" {
				height, _ := strconv.Atoi(m[7])
				clouds = append(clouds, Cloud{Coverage: m[5], Height: height * 100, Type: m[6]})
				continue
			}
			if m[2] == "" || m[2] == "///" {
				clouds = append(clouds, Cloud{Coverage: m[1], HeightMissing: true, Type: m[3]})
				continue
			}
			height, _ := strconv.Atoi(m[2])
			clouds = append(clouds, Cloud{Coverage: m[1], Height: height * 100, Type: m[3]})
		}
	}
	if !slices.Equal(clouds, metar.Clouds) {
		report.add("metar.cloud_mismatch", fmt.Sprintf("want %d layers, got %d", len(clouds), len(metar.Clouds)))
	}

	for _, token := range body {
		if m := testTempRegex.FindStringSubmatch(token); m != nil {
			temperature, _ := strconv.Atoi(m[2])
			if m[1] == "M" {
				temperature = -temperature
			}
			dewPoint, _ := strconv.Atoi(m[4])
			if m[3] == "M" {
				dewPoint = -dewPoint
			}
			if metar.Temperature == nil || metar.DewPoint == nil || *metar.Temperature != temperature || *metar.DewPoint != dewPoint {
				report.add("metar.temperature_mismatch", tokenShape(token))
			}
			break
		}
	}

	for _, token := range body {
		if m := testPressureRegex.FindStringSubmatch(token); m != nil {
			value, _ := strconv.Atoi(m[2])
			want, unit := float64(value), "hPa"
			if m[1] == "A" {
				want, unit = float64(value)/100, "inHg"
			}
			if math.Abs(metar.Pressure-want) > 0.001 || metar.PressureUnit != unit {
				report.add("metar.pressure_mismatch", tokenShape(token))
			}
			break
		}
	}

	for _, remark := range metar.Remarks {
		if remark.Description == "unknown remark code" {
			report.add("metar.remark_unknown", tokenShape(remark.Raw))
		}
	}
}

// expectedSMVisibility returns the first statute-mile visibility, joining a split "1 1/2SM"
func expectedSMVisibility(fields []string) string {
	for i, field := range fields {
		if !strings.HasSuffix(field, "SM") {
			continue
		}
		if i > 0 && strings.Contains(field, "/") && len(fields[i-1]) == 1 && fields[i-1][0] >= '1' && fields[i-1][0] <= '9' {
			return fields[i-1] + " " + field
		}
		return field
	}
	return ""
}

var tafCoverageMetrics = []string{
	"taf.station_mismatch",
	"taf.period_count_mismatch",
	"taf.weather_not_weather",
	"taf.unrepresented",
}

// TestTAFCorpusCoverage decodes the TAF corpus and counts the forecast tokens that no decoded
// field represents
func TestTAFCorpusCoverage(t *testing.T) {
	t.Parallel()

	report := newCoverageReport()
	scanner := testdata.TAF(t)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "---" {
			continue
		}
		checkTAF(line, DecodeTAF(line, corpusReference), report)
	}

	report.check(t, tafCoverageMetrics)
}

func checkTAF(line string, taf TAF, report *coverageReport) {
	fields := strings.Fields(line)
	start := headerLength(fields)
	forecast := fields[start:indexOrLen(fields, "RMK")]

	if station := expectedStation(fields); taf.Station != station {
		report.add("taf.station_mismatch", taf.Station)
	}

	periods := 1
	for i, token := range forecast {
		switch {
		case testFromRegex.MatchString(token), testProbRegex.MatchString(token), token == "BECMG", token == "INTER":
			periods++
		case token == "TEMPO":
			if i == 0 || !testProbRegex.MatchString(forecast[i-1]) {
				periods++
			}
		}
	}
	if periods != len(taf.Forecasts) {
		report.add("taf.period_count_mismatch", fmt.Sprintf("want %d periods, got %d", periods, len(taf.Forecasts)))
	}

	for _, period := range taf.Forecasts {
		for _, weather := range period.Weather {
			if !isTestWeatherGroup(weather) {
				report.add("taf.weather_not_weather", weather)
			}
		}
	}

	represented := representedTokens(taf)
	for _, period := range taf.Forecasts {
		addForecastRenderings(period, represented)
	}
	for _, token := range forecast {
		switch {
		case testFromRegex.MatchString(token), testProbRegex.MatchString(token), testPeriodRegex.MatchString(token),
			token == "TEMPO", token == "BECMG", token == "INTER":
			continue
		}
		if !represented[token] {
			report.add("taf.unrepresented", tokenShape(token))
		}
	}
}
