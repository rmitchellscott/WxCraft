package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Common weather phenomena mapping used across the application
var weatherCodes = map[string]string{
	"VC": "vicinity",
	"+":  "heavy",
	"-":  "light",
	"MI": "shallow",
	"PR": "partial",
	"BC": "patches",
	"DR": "low drifting",
	"BL": "blowing",
	"SH": "shower",
	"TS": "thunderstorm",
	"FZ": "freezing",
	"DZ": "drizzle",
	"RA": "rain",
	"SN": "snow",
	"SG": "snow grains",
	"IC": "ice crystals",
	"PL": "ice pellets",
	"GR": "hail",
	"GS": "small hail",
	"UP": "unknown precipitation",
	"BR": "mist",
	"FG": "fog",
	"FU": "smoke",
	"VA": "volcanic ash",
	"DU": "widespread dust",
	"SA": "sand",
	"HZ": "haze",
	"PY": "spray",
	"PO": "dust whirls",
	"SQ": "squalls",
	"FC": "funnel cloud",
	"SS": "sandstorm",
	"DS": "duststorm",
}

// Common cloud coverage mapping
var cloudCoverage = map[string]string{
	"SKC": "sky clear",
	"CLR": "slear",
	"FEW": "few clouds",
	"SCT": "scattered clouds",
	"BKN": "broken clouds",
	"OVC": "overcast",
}

// Common cloud type mapping
var cloudTypes = map[string]string{
	"CB":  "cumulonimbus",
	"TCU": "towering cumulus",
}

// Common weather description mapping for simplified display
var weatherDescriptions = map[string]string{
	// Basic codes
	"BR":   "mist",
	"FG":   "fog",
	"-RA":  "light rain",
	"RA":   "rain",
	"+RA":  "heavy rain",
	"-SN":  "light snow",
	"SN":   "snow",
	"+SN":  "heavy snow",
	"VCSH": "showers in vicinity",
	"VCTS": "thunderstorm in vicinity",
	"TS":   "thunderstorm",
	"TSRA": "thunderstorm with rain",
	"DZ":   "drizzle",
	"-DZ":  "light drizzle",
	"+DZ":  "heavy drizzle",

	// Composite codes - showers
	"-SHRA": "light rain showers",
	"SHRA":  "rain showers",
	"+SHRA": "heavy rain showers",
	"-SHSN": "light snow showers",
	"SHSN":  "snow showers",
	"+SHSN": "heavy snow showers",
	"SHGR":  "hail showers",
	"-SHGR": "light hail showers",
	"+SHGR": "heavy hail showers",

	// Thunderstorms
	"+TS":   "heavy thunderstorm",
	"-TS":   "light thunderstorm",
	"-TSRA": "light thunderstorm with rain",
	"+TSRA": "heavy thunderstorm with rain",
	"TSSN":  "thunderstorm with snow",
	"-TSSN": "light thunderstorm with snow",
	"+TSSN": "heavy thunderstorm with snow",
	"TSGR":  "thunderstorm with hail",
	"+TSGR": "heavy thunderstorm with hail",

	// Freezing precipitation
	"FZRA":  "freezing rain",
	"-FZRA": "light freezing rain",
	"+FZRA": "heavy freezing rain",
	"FZDZ":  "freezing drizzle",
	"-FZDZ": "light freezing drizzle",
	"+FZDZ": "heavy freezing drizzle",

	// Blowing and drifting
	"BLSN": "blowing snow",
	"DRSN": "drifting snow",
	"BLDU": "blowing dust",
	"BLSA": "blowing sand",

	// Vicinity phenomena
	"VCFG": "fog in vicinity",
	"VCFC": "funnel cloud in vicinity",

	// Other combinations
	"MIFG": "shallow fog",
	"BCFG": "patches of fog",
	"PRFG": "partial fog",
	"FC":   "funnel cloud",
	"+FC":  "tornado/waterspout",
}

// Commonly used regular expressions
var (
	timeRegex     = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
	windRegex     = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	visRegexM     = regexp.MustCompile(`^(\d+(?:/\d+)?|M)SM$`)
	visRegexP     = regexp.MustCompile(`^(\d+(?:/\d+)?|M|P)(\d+)SM$`)
	cloudRegex    = regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})?(CB|TCU)?$`)
	tempRegex     = regexp.MustCompile(`^(M?)(\d{2})/(M?)(\d{2})$`)
	pressureRegex = regexp.MustCompile(`^A(\d{4})$`)
	validRegex    = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
)

// WeatherData contains common fields for different weather reports
type WeatherData struct {
	Raw     string
	Station string
	Time    time.Time
}

// Wind represents wind information in a weather report
type Wind struct {
	Direction string
	Speed     int
	Gust      int
	Unit      string
}

// Cloud represents cloud information in a weather report
type Cloud struct {
	Coverage string
	Height   int
	Type     string // CB, TCU, etc.
}

// Remark represents a decoded remark from the RMK section
type Remark struct {
	Raw         string
	Description string
}

// METAR represents a decoded METAR weather report
type METAR struct {
	WeatherData
	Wind        Wind
	Visibility  string
	Weather     []string
	Clouds      []Cloud
	Temperature int
	DewPoint    int
	Pressure    float64
	Remarks     []Remark
}

// Forecast represents a single forecast period within a TAF
type Forecast struct {
	Type       string    // FM (from), TEMPO (temporary), BECMG (becoming), etc.
	From       time.Time // Start time of this forecast period
	To         time.Time // End time of this forecast period (if applicable)
	Wind       Wind
	Visibility string
	Weather    []string
	Clouds     []Cloud
	Raw        string // Raw text for this forecast period
}

// TAF represents a decoded Terminal Aerodrome Forecast
type TAF struct {
	WeatherData
	ValidFrom time.Time
	ValidTo   time.Time
	Forecasts []Forecast
}

// fetchData fetches data from a URL for a given station code
func fetchData(urlTemplate string, stationCode string, dataType string) (string, error) {
	url := fmt.Sprintf(urlTemplate, stationCode)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching %s: %w", dataType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	data := strings.TrimSpace(string(body))
	if data == "" {
		return "", fmt.Errorf("no %s data found for station %s", dataType, stationCode)
	}

	return data, nil
}

// FetchMETAR fetches the raw METAR for a given station code
func FetchMETAR(stationCode string) (string, error) {
	return fetchData("https://aviationweather.gov/cgi-bin/data/metar.php?ids=%s", stationCode, "METAR")
}

// FetchTAF fetches the raw TAF for a given station code
func FetchTAF(stationCode string) (string, error) {
	return fetchData("https://aviationweather.gov/cgi-bin/data/taf.php?ids=%s", stationCode, "TAF")
}

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

// DecodeMETAR decodes a raw METAR string into a METAR struct
func DecodeMETAR(raw string) METAR {
	m := METAR{WeatherData: WeatherData{Raw: raw}}
	parts := strings.Fields(raw)

	if len(parts) < 2 {
		return m
	}

	// Station code
	m.Station = parts[0]

	// Time
	if timeRegex.MatchString(parts[1]) {
		if parsedTime, err := parseTime(parts[1]); err == nil {
			m.Time = parsedTime
		}
	}

	// Find the RMK section if it exists
	rmkIndex := -1
	for i, part := range parts {
		if part == "RMK" {
			rmkIndex = i
			break
		}
	}

	// Process non-remark parts
	endIndex := len(parts)
	if rmkIndex != -1 {
		endIndex = rmkIndex
	}

	for i := 2; i < endIndex; i++ {
		part := parts[i]

		// Wind
		if windRegex.MatchString(part) {
			m.Wind = parseWind(part)
			continue
		}

		// Visibility
		if visRegexM.MatchString(part) {
			m.Visibility = part
			continue
		}

		// Weather phenomena
		if isWeatherCode(part) {
			m.Weather = append(m.Weather, part)
			continue
		}

		// Clouds
		// Clouds
		if cloudRegex.MatchString(part) {
			cloud := parseCloud(part)
			m.Clouds = append(m.Clouds, cloud)
			continue
		}

		// Temperature and dew point
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
			m.DewPoint = dewPoint
			continue
		}

		// Pressure
		if pressureRegex.MatchString(part) {
			matches := pressureRegex.FindStringSubmatch(part)
			pressureStr := matches[1]
			pressureInt, _ := strconv.Atoi(pressureStr)
			m.Pressure = float64(pressureInt) / 100.0
			continue
		}
	}

	// Process remarks if they exist
	if rmkIndex != -1 && rmkIndex+1 < len(parts) {
		m.Remarks = processRemarks(parts[rmkIndex+1:])
	}

	return m
}

// processRemarks processes the remarks section of a METAR
func processRemarks(remarkParts []string) []Remark {
	remarks := []Remark{}

	// Common remark codes and their descriptions
	remarkCodes := map[string]string{
		"AO1":    "automated station without precipitation sensor",
		"AO2":    "automated station with precipitation sensor",
		"SLP":    "sea level pressure",
		"RMK":    "remarks indicator",
		"PRESRR": "pressure rising rapidly",
		"PRESFR": "pressure falling rapidly",
		"NOSIG":  "no significant changes expected",
		"TEMPO":  "temporary",
		"BECMG":  "becoming",
		"VIRGA":  "precipitation not reaching ground",
		"FROPA":  "frontal passage",
	}

	// Process individual remarks or groups of related remarks
	i := 0
	for i < len(remarkParts) {
		part := remarkParts[i]

		// Handle peak wind
		if strings.HasPrefix(part, "PK") && i+2 < len(remarkParts) {
			windRegex := regexp.MustCompile(`^PK\s+WND\s+(\d{3})(\d{2,3})/(\d{2})(\d{2})$`)
			if windRegex.MatchString(strings.Join(remarkParts[i:i+3], " ")) {
				matches := windRegex.FindStringSubmatch(strings.Join(remarkParts[i:i+3], " "))
				dir := matches[1]
				speed := matches[2]
				hour := matches[3]
				minute := matches[4]

				remarks = append(remarks, Remark{
					Raw:         strings.Join(remarkParts[i:i+3], " "),
					Description: fmt.Sprintf("peak wind %s° at %s knots at %s:%s", dir, speed, hour, minute),
				})
				i += 3
				continue
			}
		}

		// Handle sea level pressure
		if strings.HasPrefix(part, "SLP") {
			slpValue := part[3:] // This gets "093" from "SLP093"
			slp, err := strconv.Atoi(slpValue)
			if err == nil {
				// SLP is given in tenths of millibars with an implied leading 10 or 9
				var prefix float64 = 1000.0
				if slp >= 500 {
					prefix = 900.0
				}
				slpHpa := prefix + float64(slp)/10
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("sea level pressure %.1f hPa", slpHpa),
				})
			} else {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: "sea level pressure (invalid format)",
				})
			}
			i++
			continue
		}

		// Handle Temperature/Dew Point in tenths of degrees
		tempDetailedRegex := regexp.MustCompile(`^T(\d)(\d{3})(\d)(\d{3})$`)
		if tempDetailedRegex.MatchString(part) {
			matches := tempDetailedRegex.FindStringSubmatch(part)
			tempSign := matches[1]
			tempVal := matches[2]
			dewSign := matches[3]
			dewVal := matches[4]

			temp, _ := strconv.Atoi(tempVal)
			dew, _ := strconv.Atoi(dewVal)

			// Convert to tenths of degrees
			tempF := float64(temp) / 10.0
			dewF := float64(dew) / 10.0

			if tempSign == "1" {
				tempF = -tempF
			}
			if dewSign == "1" {
				dewF = -dewF
			}

			remarks = append(remarks, Remark{
				Raw:         part,
				Description: fmt.Sprintf("temperature %.1f°C, dew point %.1f°C", tempF, dewF),
			})
			i++
			continue
		}

		// Handle 6-hour maximum temperature (format: 1sTTT)
		if len(part) == 5 && part[0] == '1' {
			sign := part[1]
			tempStr := part[2:]
			temp, err := strconv.Atoi(tempStr)
			if err == nil {
				tempValue := float64(temp) / 10.0 // Convert to degrees
				if sign == '1' {
					tempValue = -tempValue // Apply negative sign if needed
				}
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("6-hour maximum temperature %.1f°C", tempValue),
				})
				i++
				continue
			}
		}

		// Handle 6-hour minimum temperature (format: 2sTTT)
		if len(part) == 5 && part[0] == '2' {
			sign := part[1]
			tempStr := part[2:]
			temp, err := strconv.Atoi(tempStr)
			if err == nil {
				tempValue := float64(temp) / 10.0 // Convert to degrees
				if sign == '1' {
					tempValue = -tempValue // Apply negative sign if needed
				}
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("6-hour minimum temperature %.1f°C", tempValue),
				})
				i++
				continue
			}
		}

		// Handle 3-hour pressure change (format: 3PPPP)
		if len(part) == 5 && part[0] == '3' {
			pressStr := part[1:]
			press, err := strconv.Atoi(pressStr)
			if err == nil {
				hpa := float64(press) / 10.0 // Convert to hPa
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("3-hour pressure change: %.1f hPa", hpa),
				})
				i++
				continue
			}
		}

		// Handle pressure tendency (format: 5appp)
		if len(part) == 5 && part[0] == '5' {
			tendencyCode := part[1]
			changeStr := part[2:]
			change, err := strconv.Atoi(changeStr)
			if err == nil {
				changeValue := float64(change) / 10.0 // Convert to hPa

				tendencyDesc := "unknown"
				switch tendencyCode {
				case '0':
					tendencyDesc = "increasing, then decreasing"
				case '1':
					tendencyDesc = "increasing, then steady"
				case '2':
					tendencyDesc = "increasing steadily"
				case '3':
					tendencyDesc = "increasing, then increasing more rapidly"
				case '4':
					tendencyDesc = "steady"
				case '5':
					tendencyDesc = "decreasing, then increasing"
				case '6':
					tendencyDesc = "decreasing, then steady"
				case '7':
					tendencyDesc = "decreasing steadily"
				case '8':
					tendencyDesc = "decreasing, then decreasing more rapidly"
				}

				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("pressure tendency: %s, %.1f hPa change", tendencyDesc, changeValue),
				})
				i++
				continue
			}
		}

		// Handle precipitation amounts
		if precRegex := regexp.MustCompile(`^P(\d{4})$`); precRegex.MatchString(part) {
			matches := precRegex.FindStringSubmatch(part)
			precip, _ := strconv.Atoi(matches[1])
			inches := float64(precip) / 100.0

			remarks = append(remarks, Remark{
				Raw:         part,
				Description: fmt.Sprintf("precipitation of %.2f inches in the last hour", inches),
			})
			i++
			continue
		}

		// Handle 24-hour precipitation (format: 7RRRR)
		if len(part) == 5 && part[0] == '7' {
			precipStr := part[1:]
			precip, err := strconv.Atoi(precipStr)
			if err == nil {
				inches := float64(precip) / 100.0 // Convert to inches
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("24-hour precipitation: %.2f inches", inches),
				})
				i++
				continue
			}
		}

		// Handle snow depth on ground (format: 4/sss)
		if strings.HasPrefix(part, "4/") && len(part) == 5 {
			snowStr := part[2:]
			snow, err := strconv.Atoi(snowStr)
			if err == nil {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("snow depth: %d inches", snow),
				})
				i++
				continue
			}
		}

		// Handle ice accretion (format: IhVV)
		if len(part) == 5 && part[0] == 'I' && part[1] >= '1' && part[1] <= '3' {
			hourDigit := part[1]
			accretionStr := part[2:]
			accretion, err := strconv.Atoi(accretionStr)
			if err == nil {
				hours := map[byte]string{
					'1': "1-hour",
					'2': "3-hour",
					'3': "6-hour",
				}

				timeframe := hours[hourDigit]
				inches := float64(accretion) / 100.0 // Convert to inches

				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("%s ice accretion: %.2f inches", timeframe, inches),
				})
				i++
				continue
			}
		}

		// Handle recent weather (format: REww)
		if strings.HasPrefix(part, "RE") && len(part) >= 4 {
			wxType := part[2:]
			weatherMap := map[string]string{
				"RA": "rain",
				"SN": "snow",
				"GR": "hail",
				"GS": "small hail",
				"TS": "thunderstorm",
				"FG": "fog",
				"SQ": "squall",
				"FC": "funnel cloud",
			}

			if desc, ok := weatherMap[wxType]; ok {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: fmt.Sprintf("recent %s", desc),
				})
			} else {
				remarks = append(remarks, Remark{
					Raw:         part,
					Description: "recent weather phenomenon",
				})
			}
			i++
			continue
		}

		// Handle runway visual range (format: Rrrr/Vvvvft or similar)
		if strings.HasPrefix(part, "R") && strings.Contains(part, "/") {
			remarks = append(remarks, Remark{
				Raw:         part,
				Description: "runway visual range information",
			})
			i++
			continue
		}

		// Handle SNOINCR (format: SNINCR int/int)
		if part == "SNINCR" && i+1 < len(remarkParts) {
			snowData := remarkParts[i+1]
			if strings.Contains(snowData, "/") {
				parts := strings.Split(snowData, "/")
				if len(parts) == 2 {
					remarks = append(remarks, Remark{
						Raw:         part + " " + snowData,
						Description: fmt.Sprintf("snow increasing rapidly: %s inch within %s hour", parts[0], parts[1]),
					})
					i += 2
					continue
				}
			}
		}

		// Handle CIG for ceiling (format: CIG ddd)
		if part == "CIG" && i+1 < len(remarkParts) {
			if height, err := strconv.Atoi(remarkParts[i+1]); err == nil {
				remarks = append(remarks, Remark{
					Raw:         part + " " + remarkParts[i+1],
					Description: fmt.Sprintf("variable ceiling height: %d feet", height*100),
				})
				i += 2
				continue
			}
		}

		// Check for known remark codes
		if desc, ok := remarkCodes[part]; ok {
			remarks = append(remarks, Remark{
				Raw:         part,
				Description: desc,
			})
			i++
			continue
		}

		// Catch-all for unrecognized remarks
		remarks = append(remarks, Remark{
			Raw:         part,
			Description: "unknown remark code",
		})
		i++
	}

	return remarks
}

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

	// Find index of first FM, BECMG, or TEMPO
	var changeIndex int
	for i, part := range parts {
		if part == "FM" || strings.HasPrefix(part, "FM") || part == "BECMG" || part == "TEMPO" {
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
					nextPart == "BECMG" || nextPart == "TEMPO" {
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
					nextPart == "BECMG" || nextPart == "TEMPO" {
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

// CelsiusToFahrenheit converts temperature from Celsius to Fahrenheit
func CelsiusToFahrenheit(celsius int) int {
	return (celsius * 9 / 5) + 32
}

// InHgToMillibars converts pressure from inches of mercury to millibars (hPa)
func InHgToMillibars(inHg float64) float64 {
	return inHg * 33.8639
}

// Calculate the relative time string
func relativeTimeString(t time.Time) string {
	now := time.Now().UTC()
	diff := now.Sub(t)

	// Convert to minutes for easier comparisons
	minutes := int(diff.Minutes())

	if minutes < 0 {
		// For future times (rare, but possible with timezone issues)
		return "(in the future)"
	} else if minutes < 1 {
		return "(just now)"
	} else if minutes < 60 {
		return fmt.Sprintf("(%d minutes ago)", minutes)
	} else if minutes < 1440 { // less than 24 hours
		hours := minutes / 60
		mins := minutes % 60
		if mins == 0 {
			return fmt.Sprintf("(%d hours ago)", hours)
		}
		return fmt.Sprintf("(%d hours, %d minutes ago)", hours, mins)
	} else {
		days := minutes / 1440
		hours := (minutes % 1440) / 60
		if hours == 0 {
			return fmt.Sprintf("(%d days ago)", days)
		}
		return fmt.Sprintf("(%d days, %d hours ago)", days, hours)
	}
}

// formatVisibility converts raw visibility string to human-readable format
func formatVisibility(visibility string) string {
	if visibility == "" {
		return ""
	}

	// Decode common visibility formats
	if visibility == "P6SM" {
		return "Greater than 6 statute miles"
	} else if strings.HasSuffix(visibility, "SM") {
		// Check for fractions
		if strings.Contains(visibility, "/") {
			// Handle fractional values like "1/2SM"
			return visibility[:len(visibility)-2] + " statute miles"
		} else if strings.HasPrefix(visibility, "M") {
			// M prefix means "less than"
			value := visibility[1 : len(visibility)-2]
			return "Less than " + value + " statute miles"
		} else {
			// Regular integer values like "1SM" or "6SM"
			value := visibility[:len(visibility)-2]
			return value + " statute miles"
		}
	}

	return visibility
}

// formatWind converts a Wind struct to a human-readable string
func formatWind(wind Wind) string {
	if wind.Speed == 0 && wind.Direction == "" {
		return ""
	}

	windStr := ""
	if wind.Direction == "VRB" {
		windStr += "Variable"
	} else if wind.Direction != "" {
		windStr += fmt.Sprintf("From %s°", wind.Direction)
	}

	if wind.Speed > 0 {
		windStr += fmt.Sprintf(" at %d knots", wind.Speed)
		if wind.Gust > 0 {
			windStr += fmt.Sprintf(", gusting to %d knots", wind.Gust)
		}
	}

	return windStr
}

// formatClouds converts a slice of Cloud structs to a human-readable string
func formatClouds(clouds []Cloud) string {
	if len(clouds) == 0 {
		return ""
	}

	var cloudStrs []string
	for _, cloud := range clouds {
		coverStr := cloud.Coverage
		if c, ok := cloudCoverage[cloud.Coverage]; ok {
			coverStr = c
		}

		cloudDesc := coverStr
		if cloud.Height > 0 {
			cloudDesc = fmt.Sprintf("%s at %s feet", coverStr, formatNumberWithCommas(cloud.Height))
		}

		if cloud.Type != "" {
			typeDesc := cloud.Type
			if t, ok := cloudTypes[cloud.Type]; ok {
				typeDesc = t
			}
			cloudDesc = fmt.Sprintf("%s (%s)", cloudDesc, typeDesc)
		}

		cloudStrs = append(cloudStrs, cloudDesc)
	}

	return strings.Join(cloudStrs, ", ")
}

// formatWeather converts a slice of weather strings to a human-readable format
func formatWeather(weather []string) string {
	if len(weather) == 0 {
		return ""
	}

	var weatherStrs []string
	for _, wx := range weather {
		if desc, ok := weatherDescriptions[wx]; ok {
			weatherStrs = append(weatherStrs, desc)
		} else {
			weatherStrs = append(weatherStrs, wx)
		}
	}

	return strings.Join(weatherStrs, ", ")
}

// / FormatMETAR formats a METAR struct for display
func FormatMETAR(m METAR) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Station: %s\n", m.Station))

	if !m.Time.IsZero() {
		relTime := relativeTimeString(m.Time)
		sb.WriteString(fmt.Sprintf("Time: %s %s\n", m.Time.Format("2006-01-02 15:04 UTC"), relTime))
	}

	// Wind
	windStr := formatWind(m.Wind)
	if windStr != "" {
		sb.WriteString("Wind: " + windStr + "\n")
	}

	// Visibility
	visibilityDesc := formatVisibility(m.Visibility)
	if visibilityDesc != "" {
		sb.WriteString(fmt.Sprintf("Visibility: %s\n", visibilityDesc))
	}

	// Check if we have only CLR clouds
	hasClear := false
	hasOnlyClear := true
	var cloudsWithHeight []Cloud

	for _, cloud := range m.Clouds {
		if cloud.Coverage == "CLR" || cloud.Coverage == "SKC" {
			hasClear = true
		} else {
			hasOnlyClear = false
		}

		// Collect clouds with height for possible display
		if cloud.Height > 0 {
			cloudsWithHeight = append(cloudsWithHeight, cloud)
		}
	}

	// Weather
	if len(m.Weather) > 0 {
		weatherStr := formatWeather(m.Weather)
		sb.WriteString(fmt.Sprintf("Weather: %s\n", capitalizeFirst(weatherStr)))
	} else if hasClear {
		// No weather but we have CLR or SKC, so show "Clear" as the weather
		sb.WriteString("Weather: Clear\n")
	}

	// Clouds - only show if we have clouds other than CLR/SKC
	// Or if we have clouds with height information
	if !hasOnlyClear || len(cloudsWithHeight) > 0 {
		// Filter out CLR/SKC from display if we already showed it in Weather
		var cloudsToDisplay []Cloud
		if hasClear && len(m.Weather) == 0 {
			// We're showing "Clear" in Weather, so only show non-CLR/SKC clouds
			for _, cloud := range m.Clouds {
				if cloud.Coverage != "CLR" && cloud.Coverage != "SKC" {
					cloudsToDisplay = append(cloudsToDisplay, cloud)
				}
			}
		} else {
			cloudsToDisplay = m.Clouds
		}

		if len(cloudsToDisplay) > 0 {
			cloudStr := formatClouds(cloudsToDisplay)
			sb.WriteString(fmt.Sprintf("Clouds: %s\n", capitalizeFirst(cloudStr)))
		}
	}

	// Temperature with Fahrenheit conversion
	tempF := CelsiusToFahrenheit(m.Temperature)
	sb.WriteString(fmt.Sprintf("Temperature: %d°C | %d°F\n", m.Temperature, tempF))

	// Dew point with Fahrenheit conversion
	dewPointF := CelsiusToFahrenheit(m.DewPoint)
	sb.WriteString(fmt.Sprintf("Dew Point: %d°C | %d°F\n", m.DewPoint, dewPointF))

	// Pressure with millibar conversion
	if m.Pressure > 0 {
		pressureMb := InHgToMillibars(m.Pressure)
		sb.WriteString(fmt.Sprintf("Pressure: %.2f inHg | %.1f mbar\n", m.Pressure, pressureMb))
	}

	// Remarks
	if len(m.Remarks) > 0 {
		sb.WriteString("\nRemarks:\n")
		for _, remark := range m.Remarks {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", remark.Raw, capitalizeFirst(remark.Description)))
		}
	}

	return sb.String()
}

// FormatTAF formats a TAF struct for display
func FormatTAF(t TAF) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Station: %s\n", t.Station))

	if !t.Time.IsZero() {
		relTime := relativeTimeString(t.Time)
		sb.WriteString(fmt.Sprintf("Issued: %s %s\n", t.Time.Format("2006-01-02 15:04 UTC"), relTime))
	}

	if !t.ValidFrom.IsZero() && !t.ValidTo.IsZero() {
		sb.WriteString(fmt.Sprintf("Valid: %s to %s\n",
			t.ValidFrom.Format("2006-01-02 15:04 UTC"),
			t.ValidTo.Format("2006-01-02 15:04 UTC")))
	}

	sb.WriteString("\nForecast Periods:\n")

	for i, forecast := range t.Forecasts {
		// Format the forecast type
		var periodType string
		switch forecast.Type {
		case "BASE":
			periodType = "Base Forecast"
		case "FM":
			periodType = "From"
		case "TEMPO":
			periodType = "Temporary"
		case "BECMG":
			periodType = "Becoming"
		default:
			periodType = forecast.Type
		}

		sb.WriteString(fmt.Sprintf("\n%d. %s", i+1, periodType))

		// Time period
		if !forecast.From.IsZero() {
			if forecast.To.IsZero() {
				sb.WriteString(fmt.Sprintf(" %s until end of forecast",
					forecast.From.Format("2006-01-02 15:04 UTC")))
			} else {
				sb.WriteString(fmt.Sprintf(" %s to %s",
					forecast.From.Format("2006-01-02 15:04 UTC"),
					forecast.To.Format("2006-01-02 15:04 UTC")))
			}
		}
		sb.WriteString("\n")

		// Wind
		windStr := formatWind(forecast.Wind)
		if windStr != "" {
			sb.WriteString(fmt.Sprintf("   Wind: %s\n", windStr))
		}

		// Visibility
		visibilityDesc := formatVisibility(forecast.Visibility)
		if visibilityDesc != "" {
			sb.WriteString(fmt.Sprintf("   Visibility: %s\n", visibilityDesc))
		}

		// Weather
		weatherStr := formatWeather(forecast.Weather)
		if weatherStr != "" {
			sb.WriteString(fmt.Sprintf("   Weather: %s\n", capitalizeFirst(weatherStr)))
		}

		// Clouds
		cloudStr := formatClouds(forecast.Clouds)
		if cloudStr != "" {
			sb.WriteString(fmt.Sprintf("   Clouds: %s\n", capitalizeFirst(cloudStr)))
		}
	}

	return sb.String()
}

// // readFromStdin reads data from stdin if available
// func readFromStdin() (string, string, bool) {
// 	// Check if input is being piped in (stdin)
// 	info, err := os.Stdin.Stat()
// 	stdinHasData := (err == nil && info.Mode()&os.ModeCharDevice == 0)

// 	if !stdinHasData {
// 		return "", "", false
// 	}

// 	// Read from stdin if data is piped in
// 	scanner := bufio.NewScanner(os.Stdin)
// 	if scanner.Scan() {
// 		rawInput := scanner.Text()

// 		// Try to extract station code from the raw input
// 		parts := strings.Fields(rawInput)
// 		if len(parts) > 0 {
// 			return parts[0], rawInput, true
// 		}
// 	}

// 	return "", "", false
// }

// // getStationCodeFromArgs gets station code from command-line args
// func getStationCodeFromArgs(args []string) (string, error) {
// 	if len(args) < 1 {
// 		return "", fmt.Errorf("no station code provided")
// 	}

// 	stationCode := strings.ToUpper(strings.TrimSpace(args[0]))
// 	if len(stationCode) != 4 {
// 		return "", fmt.Errorf("invalid station code: must be 4 characters")
// 	}

// 	return stationCode, nil
// }

func main() {
	// Define command-line flags
	metarOnly := flag.Bool("metar", false, "Show only METAR")
	tafOnly := flag.Bool("taf", false, "Show only TAF")
	noRawFlag := flag.Bool("no-raw", false, "Hide raw data")
	flag.Parse()

	// First check stdin for piped data
	stationCode, rawInput, stdinHasData := readFromStdin()

	// If no stdin data, get station code from args or prompt
	if !stdinHasData {
		var err error

		// Try command line args first
		remainingArgs := flag.Args()
		if len(remainingArgs) > 0 {
			stationCode, err = getStationCodeFromArgs(remainingArgs)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		} else {
			// Prompt the user
			stationCode, err = promptForStationCode()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		}
	}

	// Fetch and display METAR if requested or by default
	if !*tafOnly {
		processMETAR(stationCode, rawInput, stdinHasData, *noRawFlag)
	}

	// Fetch and display TAF if requested or by default
	if !*metarOnly && !stdinHasData {
		// Add a line break if we also displayed METAR
		if !*tafOnly {
			fmt.Println("\n----------------------------------\n")
		}

		processTAF(stationCode, *noRawFlag)
	}
}
