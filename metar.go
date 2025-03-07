package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// METAR represents a decoded METAR weather report
type METAR struct {
	Raw         string
	Station     string
	Time        time.Time
	Wind        Wind
	Visibility  string
	Weather     []string
	Clouds      []Cloud
	Temperature int
	DewPoint    int
	Pressure    float64
	Remarks     []Remark
}

// Wind represents wind information in a METAR
type Wind struct {
	Direction string
	Speed     int
	Gust      int
	Unit      string
}

// Cloud represents cloud information in a METAR
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

// TAF represents a decoded Terminal Aerodrome Forecast
type TAF struct {
	Raw      string
	Station  string
	IssuedAt time.Time
	ValidFrom time.Time
	ValidTo   time.Time
	Forecasts []Forecast
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
	Raw        string    // Raw text for this forecast period
}

// FetchMETAR fetches the raw METAR for a given station code
func FetchMETAR(stationCode string) (string, error) {
	url := fmt.Sprintf("https://aviationweather.gov/cgi-bin/data/metar.php?ids=%s", stationCode)
	
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching METAR: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}
	
	metar := strings.TrimSpace(string(body))
	if metar == "" {
		return "", fmt.Errorf("no METAR data found for station %s", stationCode)
	}
	
	return metar, nil
}

// FetchTAF fetches the raw TAF for a given station code
func FetchTAF(stationCode string) (string, error) {
	url := fmt.Sprintf("https://aviationweather.gov/cgi-bin/data/taf.php?ids=%s", stationCode)
	
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching TAF: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}
	
	taf := strings.TrimSpace(string(body))
	if taf == "" {
		return "", fmt.Errorf("no TAF data found for station %s", stationCode)
	}
	
	return taf, nil
}

// DecodeMETAR decodes a raw METAR string into a METAR struct
func DecodeMETAR(raw string) METAR {
	m := METAR{Raw: raw}
	parts := strings.Fields(raw)
	
	if len(parts) < 2 {
		return m
	}
	
	// Station code
	m.Station = parts[0]
	
	// Time
	if timeRegex := regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`); timeRegex.MatchString(parts[1]) {
		matches := timeRegex.FindStringSubmatch(parts[1])
		day, _ := strconv.Atoi(matches[1])
		hour, _ := strconv.Atoi(matches[2])
		minute, _ := strconv.Atoi(matches[3])
		
		// Use current year and month
		now := time.Now().UTC()
		m.Time = time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, time.UTC)
		
		// Handle month rollover
		if now.Day() < day {
			m.Time = m.Time.AddDate(0, -1, 0)
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
		if windRegex := regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`); windRegex.MatchString(part) {
			matches := windRegex.FindStringSubmatch(part)
			m.Wind.Direction = matches[1]
			m.Wind.Speed, _ = strconv.Atoi(matches[2])
			if matches[4] != "" {
				m.Wind.Gust, _ = strconv.Atoi(matches[4])
			}
			m.Wind.Unit = "KT"
			continue
		}
		
		// Visibility
		if visRegex := regexp.MustCompile(`^(\d+(?:/\d+)?|M)SM$`); visRegex.MatchString(part) {
			m.Visibility = part
			continue
		}
		
		// Weather phenomena
		weatherCodes := map[string]string{
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
		
		if !strings.HasPrefix(part, "FEW") && 
   !strings.HasPrefix(part, "SCT") && 
   !strings.HasPrefix(part, "BKN") && 
   !strings.HasPrefix(part, "OVC") && 
   !strings.HasPrefix(part, "SKC") && 
   !strings.HasPrefix(part, "CLR") {
    
    hasWeatherCode := false
    for code := range weatherCodes {
        if strings.Contains(part, code) {
            hasWeatherCode = true
            break
        }
    }
    
    if hasWeatherCode {
        m.Weather = append(m.Weather, part)
        continue
    }
}
		
		// Clouds
		if cloudRegex := regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})(CB|TCU)?$`); cloudRegex.MatchString(part) {

			matches := cloudRegex.FindStringSubmatch(part)
			cloud := Cloud{
				Coverage: matches[1],
				Type:     matches[3],
			}
			if matches[2] != "" {
				height, _ := strconv.Atoi(matches[2])
				cloud.Height = height * 100
			}
			m.Clouds = append(m.Clouds, cloud)
			continue
		}
		
		// Temperature and dew point
		if tempRegex := regexp.MustCompile(`^(M?)(\d{2})/(M?)(\d{2})$`); tempRegex.MatchString(part) {
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
		if pressureRegex := regexp.MustCompile(`^A(\d{4})$`); pressureRegex.MatchString(part) {
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
			if windRegex := regexp.MustCompile(`^PK\s+WND\s+(\d{3})(\d{2,3})/(\d{2})(\d{2})$`); windRegex.MatchString(strings.Join(remarkParts[i:i+3], " ")) {
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
				// If SLP is 982.5, it would be reported as 825 -> 982.5
				// If SLP is 1013.2, it would be reported as 132 -> 1013.2
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
		if tempRegex := regexp.MustCompile(`^T(\d)(\d{3})(\d)(\d{3})$`); tempRegex.MatchString(part) {
			matches := tempRegex.FindStringSubmatch(part)
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

		// Handle hourly temperature and dew point (format: TsnTTTsnTTT)
		if len(part) > 5 && part[0] == 'T' {
			// Already handled with more complex regex above
			// This is just a reminder that T codes are important
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
					Description: fmt.Sprintf("variable ceiling height: %d feet", height * 100),
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
    t := TAF{Raw: raw}
    
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
    timeRegex := regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
    
    for i := startIdx + 1; i < len(parts); i++ {
        if timeRegex.MatchString(parts[i]) {
            matches := timeRegex.FindStringSubmatch(parts[i])
            day, _ := strconv.Atoi(matches[1])
            hour, _ := strconv.Atoi(matches[2])
            minute, _ := strconv.Atoi(matches[3])
            
            // Use current year and month
            now := time.Now().UTC()
            t.IssuedAt = time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, time.UTC)
            
            // Handle month rollover
            if now.Day() < day {
                t.IssuedAt = t.IssuedAt.AddDate(0, -1, 0)
            }
            continue
        }
        
        // Parse valid time period
        validRegex := regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
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
    if windRegex := regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`); windRegex.MatchString(part) {
        matches := windRegex.FindStringSubmatch(part)
        forecast.Wind.Direction = matches[1]
        forecast.Wind.Speed, _ = strconv.Atoi(matches[2])
        if matches[4] != "" {
            forecast.Wind.Gust, _ = strconv.Atoi(matches[4])
        }
        forecast.Wind.Unit = "KT"
        return
    }
    
    // Visibility
    if visRegex := regexp.MustCompile(`^(\d+(?:/\d+)?|M|P)(\d+)SM$`); visRegex.MatchString(part) {
        forecast.Visibility = part
        return
    }
    
    // Weather phenomena
    weatherCodes := map[string]bool{
        "VC": true, "+": true, "-": true, "MI": true, "PR": true, "BC": true,
        "DR": true, "BL": true, "SH": true, "TS": true, "FZ": true, "DZ": true,
        "RA": true, "SN": true, "SG": true, "IC": true, "PL": true, "GR": true,
        "GS": true, "UP": true, "BR": true, "FG": true, "FU": true, "VA": true,
        "DU": true, "SA": true, "HZ": true, "PY": true, "PO": true, "SQ": true,
        "FC": true, "SS": true, "DS": true,
    }
    
    hasWeatherCode := false
    for code := range weatherCodes {
        if strings.Contains(part, code) {
            hasWeatherCode = true
            break
        }
    }
    
    if hasWeatherCode {
        forecast.Weather = append(forecast.Weather, part)
        return
    }
    
    // Clouds
    if cloudRegex := regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})(CB|TCU)?$`); cloudRegex.MatchString(part) {
        matches := cloudRegex.FindStringSubmatch(part)
        cloud := Cloud{
            Coverage: matches[1],
            Type:     matches[3],
        }
        if matches[2] != "" {
            height, _ := strconv.Atoi(matches[2])
            cloud.Height = height * 100
        }
        forecast.Clouds = append(forecast.Clouds, cloud)
        return
    }
}

// parseForecastElements parses weather elements for a forecast period
func parseForecastElements(forecast *Forecast, parts []string) {
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		
		// Wind
		if windRegex := regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`); windRegex.MatchString(part) {
			matches := windRegex.FindStringSubmatch(part)
			forecast.Wind.Direction = matches[1]
			forecast.Wind.Speed, _ = strconv.Atoi(matches[2])
			if matches[4] != "" {
				forecast.Wind.Gust, _ = strconv.Atoi(matches[4])
			}
			forecast.Wind.Unit = "KT"
			continue
		}
		
		// Visibility
		if visRegex := regexp.MustCompile(`^(\d+(?:/\d+)?|M)SM$`); visRegex.MatchString(part) {
			forecast.Visibility = part
			continue
		}
		
		// Weather phenomena - similar to METAR processing
		weatherCodes := map[string]string{
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
		
		hasWeatherCode := false
		for code := range weatherCodes {
			if strings.Contains(part, code) {
				hasWeatherCode = true
				break
			}
		}
		
		if hasWeatherCode {
			forecast.Weather = append(forecast.Weather, part)
			continue
		}
		
		// Clouds
		if cloudRegex := regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})(CB|TCU)?$`); cloudRegex.MatchString(part) {
			matches := cloudRegex.FindStringSubmatch(part)
			cloud := Cloud{
				Coverage: matches[1],
				Type:     matches[3],
			}
			if matches[2] != "" {
				height, _ := strconv.Atoi(matches[2])
				cloud.Height = height * 100
			}
			forecast.Clouds = append(forecast.Clouds, cloud)
			continue
		}
	}
}

// CelsiusToFahrenheit converts temperature from Celsius to Fahrenheit
func CelsiusToFahrenheit(celsius int) int {
	return (celsius * 9 / 5) + 32
}

// InHgToMillibars converts pressure from inches of mercury to millibars (hPa)
func InHgToMillibars(inHg float64) float64 {
	return inHg * 33.8639
}// Calculate the relative time string
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
// FormatMETAR formats a METAR struct for display
func FormatMETAR(m METAR) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Station: %s\n", m.Station))
	
	if !m.Time.IsZero() {
		sb.WriteString(fmt.Sprintf("Time: %s\n", m.Time.Format("2006-01-02 15:04 UTC")))
	}
	
	// Wind
	if m.Wind.Speed > 0 || m.Wind.Direction != "" {
		windStr := "Wind: "
		if m.Wind.Direction == "VRB" {
			windStr += "Variable"
		} else if m.Wind.Direction != "" {
			windStr += fmt.Sprintf("From %s°", m.Wind.Direction)
		}
		
		if m.Wind.Speed > 0 {
			windStr += fmt.Sprintf(" at %d knots", m.Wind.Speed)
			if m.Wind.Gust > 0 {
				windStr += fmt.Sprintf(", gusting to %d knots", m.Wind.Gust)
			}
		}
		sb.WriteString(windStr + "\n")
	}
	
	// Visibility
	if m.Visibility != "" {
		sb.WriteString(fmt.Sprintf("Visibility: %s\n", m.Visibility))
	}
	
	// Weather
	if len(m.Weather) > 0 {
		sb.WriteString(fmt.Sprintf("Weather: %s\n", strings.Join(m.Weather, ", ")))
	}
	
	// Clouds
	if len(m.Clouds) > 0 {
		var cloudStrs []string
		for _, cloud := range m.Clouds {
			coverage := map[string]string{
				"SKC": "sky clear",
				"CLR": "clear",
				"FEW": "few clouds",
				"SCT": "scattered clouds",
				"BKN": "broken clouds",
				"OVC": "overcast",
			}
			
			coverStr := cloud.Coverage
			if c, ok := coverage[cloud.Coverage]; ok {
				coverStr = c
			}
			
			cloudDesc := coverStr
			if cloud.Height > 0 {
				cloudDesc = fmt.Sprintf("%s at %d feet", cloudDesc, cloud.Height)
			}
			
			if cloud.Type != "" {
				cloudTypes := map[string]string{
					"CB":  "cumulonimbus",
					"TCU": "towering cumulus",
				}
				typeDesc := cloud.Type
				if t, ok := cloudTypes[cloud.Type]; ok {
					typeDesc = t
				}
				cloudDesc = fmt.Sprintf("%s (%s)", cloudDesc, typeDesc)
			}
			
			cloudStrs = append(cloudStrs, cloudDesc)
		}
		sb.WriteString(fmt.Sprintf("Clouds: %s\n", strings.Join(cloudStrs, ", ")))
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
			sb.WriteString(fmt.Sprintf("  %s: %s\n", remark.Raw, remark.Description))
		}
	}
	
	return sb.String()
}

// FormatTAF formats a TAF struct for display
func FormatTAF(t TAF) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Station: %s\n", t.Station))
	
	if !t.IssuedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("Issued: %s\n", t.IssuedAt.Format("2006-01-02 15:04 UTC")))
	}
	
	if !t.ValidFrom.IsZero() && !t.ValidTo.IsZero() {
		sb.WriteString(fmt.Sprintf("Valid: %s to %s\n", 
			t.ValidFrom.Format("2006-01-02 15:04 UTC"),
			t.ValidTo.Format("2006-01-02 15:04 UTC")))
	}
	
	sb.WriteString("\nForecast Periods:\n")
	
	for i, forecast := range t.Forecasts {
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
		if forecast.Wind.Speed > 0 || forecast.Wind.Direction != "" {
			windStr := "   Wind: "
			if forecast.Wind.Direction == "VRB" {
				windStr += "Variable"
			} else if forecast.Wind.Direction != "" {
				windStr += fmt.Sprintf("From %s°", forecast.Wind.Direction)
			}
			
			if forecast.Wind.Speed > 0 {
				windStr += fmt.Sprintf(" at %d knots", forecast.Wind.Speed)
				if forecast.Wind.Gust > 0 {
					windStr += fmt.Sprintf(", gusting to %d knots", forecast.Wind.Gust)
				}
			}
			sb.WriteString(windStr + "\n")
		}
		
		// Visibility
if forecast.Visibility != "" {
    visibilityDesc := forecast.Visibility
    
    // Decode common visibility formats
    if visibilityDesc == "P6SM" {
        visibilityDesc = "Greater than 6 statute miles"
    } else if strings.HasSuffix(visibilityDesc, "SM") {
        // Check for fractions
        if strings.Contains(visibilityDesc, "/") {
            // Handle fractional values like "1/2SM"
            visibilityDesc = visibilityDesc[:len(visibilityDesc)-2] + " statute miles"
        } else if strings.HasPrefix(visibilityDesc, "M") {
            // M prefix means "less than"
            value := visibilityDesc[1:len(visibilityDesc)-2]
            visibilityDesc = "Less than " + value + " statute miles"
        } else {
            // Regular integer values like "1SM"
            value := visibilityDesc[:len(visibilityDesc)-2]
            visibilityDesc = value + " statute miles"
        }
    }
    
    sb.WriteString(fmt.Sprintf("   Visibility: %s\n", visibilityDesc))
}
		
		// Weather
		if len(forecast.Weather) > 0 {
			// Map of weather codes to human-readable descriptions
			weatherDescriptions := map[string]string{
				"VCSH": "showers in the vicinity",
				"BR": "mist",
				"+RA": "heavy rain",
				"-RA": "light rain",
				"RA": "rain",
				"SN": "snow",
				"FG": "fog",
				"TS": "thunderstorm",
				// Add more mappings as needed
			}
    
    var weatherStrs []string
    for _, wx := range forecast.Weather {
        if desc, ok := weatherDescriptions[wx]; ok {
            weatherStrs = append(weatherStrs, desc)
        } else {
            weatherStrs = append(weatherStrs, wx)
        }
    }
    
    sb.WriteString(fmt.Sprintf("   Weather: %s\n", strings.Join(weatherStrs, ", ")))
}
		// Clouds
		if len(forecast.Clouds) > 0 {
			var cloudStrs []string
			for _, cloud := range forecast.Clouds {
				coverage := map[string]string{
					"SKC": "sky clear",
					"CLR": "clear",
					"FEW": "few clouds",
					"SCT": "scattered clouds",
					"BKN": "broken clouds",
					"OVC": "overcast",
				}
				
				coverStr := cloud.Coverage
				if c, ok := coverage[cloud.Coverage]; ok {
					coverStr = c
				}
				
				cloudDesc := coverStr
				if cloud.Height > 0 {
					cloudDesc = fmt.Sprintf("%s at %d feet", cloudDesc, cloud.Height)
				}
				
				if cloud.Type != "" {
					cloudTypes := map[string]string{
						"CB":  "cumulonimbus",
						"TCU": "towering cumulus",
					}
					typeDesc := cloud.Type
					if t, ok := cloudTypes[cloud.Type]; ok {
						typeDesc = t
					}
					cloudDesc = fmt.Sprintf("%s (%s)", cloudDesc, typeDesc)
				}
				
				cloudStrs = append(cloudStrs, cloudDesc)
			}
			sb.WriteString(fmt.Sprintf("   Clouds: %s\n", strings.Join(cloudStrs, ", ")))
		}
	}
	
	return sb.String()
}

// GetStationCode gets the station code from command-line arguments or user input
func GetStationCode() (string, error) {
	// Check if provided as command-line argument
	if len(os.Args) > 1 {
		stationCode := strings.ToUpper(strings.TrimSpace(os.Args[1]))
		if len(stationCode) != 4 {
			return "", fmt.Errorf("invalid station code: must be 4 characters")
		}
		return stationCode, nil
	}
	
	// Otherwise prompt the user
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ICAO airport code (e.g., KJFK, EGLL): ")
	stationCode, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}
	
	stationCode = strings.ToUpper(strings.TrimSpace(stationCode))
	if len(stationCode) != 4 {
		return "", fmt.Errorf("invalid station code: must be 4 characters")
	}
	
	return stationCode, nil
}

// main
func main() {
	// Define command-line flags
	metarOnly := flag.Bool("metar", false, "Show only METAR")
	tafOnly := flag.Bool("taf", false, "Show only TAF")
	noRawFlag := flag.Bool("no-raw", false, "Hide raw data")
	flag.Parse()
	
	// Check if input is being piped in (stdin)
	info, err := os.Stdin.Stat()
	stdinHasData := (err == nil && info.Mode()&os.ModeCharDevice == 0)
	
	var stationCode string
	var rawInput string
	
	if stdinHasData {
		// Read from stdin if data is piped in
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			rawInput = scanner.Text()
		}
		
		// Try to extract station code from the raw input
		parts := strings.Fields(rawInput)
		if len(parts) > 0 {
			stationCode = parts[0]
		} else {
			fmt.Println("Error: Could not parse input data")
			return
		}
	} else {
		// Get station code from command line args or user input
		remainingArgs := flag.Args()
		
		if len(remainingArgs) > 0 {
			stationCode = strings.ToUpper(strings.TrimSpace(remainingArgs[0]))
			if len(stationCode) != 4 {
				fmt.Println("Error: Invalid station code. Must be 4 characters.")
				return
			}
		} else {
			// Prompt for station code
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter ICAO airport code (e.g., KJFK, EGLL): ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				return
			}
			
			stationCode = strings.ToUpper(strings.TrimSpace(input))
			if len(stationCode) != 4 {
				fmt.Println("Error: Invalid station code. Must be 4 characters.")
				return
			}
		}
	}
	
	// Fetch and display METAR if requested or by default
	if !*tafOnly {
		var metar string
		var err error
		
		if stdinHasData && rawInput != "" {
			// Use the piped data
			metar = rawInput
		} else {
			// Fetch from the service
			fmt.Printf("Fetching METAR for %s...\n", stationCode)
			metar, err = FetchMETAR(stationCode)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		}
		
		// Show raw METAR by default, unless --no-raw flag is used
		if !*noRawFlag {
			fmt.Println("\nRaw METAR:")
			fmt.Println(metar)
		}
		
		fmt.Println("\nDecoded METAR:")
		decoded := DecodeMETAR(metar)
		fmt.Print(FormatMETAR(decoded))
	}
	
	// Fetch and display TAF if requested or by default
	if !*metarOnly && !stdinHasData {
		// Add a line break if we also displayed METAR
		if !*tafOnly {
			fmt.Println("\n----------------------------------\n")
		}
		
		fmt.Printf("Fetching TAF for %s...\n", stationCode)
		
		taf, err := FetchTAF(stationCode)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			// Show raw TAF by default, unless --no-raw flag is used
			if !*noRawFlag {
				fmt.Println("\nRaw TAF:")
				fmt.Println(taf)
			}
			
			fmt.Println("\nDecoded TAF:")
			decoded := DecodeTAF(taf)
			fmt.Print(FormatTAF(decoded))
		}
	}
}
