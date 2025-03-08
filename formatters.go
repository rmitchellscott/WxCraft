package main

import (
	"fmt"
	"strconv"
	"strings"
)

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

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatNumberWithCommas adds thousands separators to a number
func formatNumberWithCommas(n int) string {
	// Convert to string first
	numStr := strconv.Itoa(n)

	// Add commas for thousands
	result := ""
	for i, c := range numStr {
		if i > 0 && (len(numStr)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}

	return result
}
