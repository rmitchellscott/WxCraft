package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Color definitions using fatih/color
var (
	labelColor      = color.New(color.FgCyan)
	valueColor      = color.New(color.FgWhite)
	dateColor       = color.New(color.FgGreen)
	sectionColor    = color.New(color.FgMagenta)
	numberColor     = color.New(color.FgGreen)
	remarkCodeColor = color.New(color.FgGreen)

	// Age-based colors
	freshColor   = color.New(color.FgGreen)
	warningColor = color.New(color.FgYellow)
	expiredColor = color.New(color.FgRed)
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

// getAgeColor returns the appropriate color based on METAR age
func getMetarAgeColor(t time.Time) *color.Color {
	minutes := int(time.Since(t).Minutes())
	if minutes > 60 {
		return expiredColor
	} else if minutes > 30 {
		return warningColor
	}
	return freshColor
}

// getTafAgeColor returns the appropriate color based on TAF age
func getTafAgeColor(t time.Time) *color.Color {
	hours := time.Since(t).Hours()
	if hours > 6.0 {
		return expiredColor
	} else if hours > 5.5 {
		return warningColor
	}
	return freshColor
}

// FormatMETAR formats a METAR struct for display with colors
func FormatMETAR(m METAR) string {
	var sb strings.Builder

	// Station
	labelColor.Fprint(&sb, "Station: ")
	sb.WriteString(m.Station + "\n")

	// Time
	if !m.Time.IsZero() {
		relTime := relativeTimeString(m.Time)
		ageColor := getMetarAgeColor(m.Time)

		labelColor.Fprint(&sb, "Time: ")
		dateColor.Fprint(&sb, m.Time.Format("2006-01-02 15:04 UTC"))
		sb.WriteString(" ")
		ageColor.Fprint(&sb, relTime)
		sb.WriteString("\n")
	}

	// Wind
	windStr := formatWind(m.Wind)
	if windStr != "" {
		labelColor.Fprint(&sb, "Wind: ")
		sb.WriteString(windStr + "\n")
	}

	// Visibility
	visibilityDesc := formatVisibility(m.Visibility)
	if visibilityDesc != "" {
		labelColor.Fprint(&sb, "Visibility: ")
		sb.WriteString(visibilityDesc + "\n")
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
		labelColor.Fprint(&sb, "Weather: ")
		sb.WriteString(capitalizeFirst(weatherStr) + "\n")
	} else if hasClear {
		// No weather but we have CLR or SKC, so show "Clear" as the weather
		labelColor.Fprint(&sb, "Weather: ")
		sb.WriteString("Clear\n")
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
			labelColor.Fprint(&sb, "Clouds: ")
			sb.WriteString(capitalizeFirst(cloudStr) + "\n")
		}
	}

	// Temperature with Fahrenheit conversion
	tempF := CelsiusToFahrenheit(m.Temperature)
	labelColor.Fprint(&sb, "Temperature: ")
	sb.WriteString(fmt.Sprintf("%d°C | %d°F\n", m.Temperature, tempF))

	// Dew point with Fahrenheit conversion
	dewPointF := CelsiusToFahrenheit(m.DewPoint)
	labelColor.Fprint(&sb, "Dew Point: ")
	sb.WriteString(fmt.Sprintf("%d°C | %d°F\n", m.DewPoint, dewPointF))

	// Pressure with millibar conversion
	if m.Pressure > 0 {
		pressureMb := InHgToMillibars(m.Pressure)
		labelColor.Fprint(&sb, "Pressure: ")
		sb.WriteString(fmt.Sprintf("%.2f inHg | %.1f mbar\n", m.Pressure, pressureMb))
	}

	// Remarks
	if len(m.Remarks) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Remarks:")
		for _, remark := range m.Remarks {
			sb.WriteString("  ")
			remarkCodeColor.Fprint(&sb, remark.Raw+": ")
			sb.WriteString(capitalizeFirst(remark.Description) + "\n")
		}
	}

	return sb.String()
}

// FormatTAF formats a TAF struct for display with colors
func FormatTAF(t TAF) string {
	var sb strings.Builder

	// Station
	labelColor.Fprint(&sb, "Station: ")
	sb.WriteString(t.Station + "\n")

	// Issued time
	if !t.Time.IsZero() {
		relTime := relativeTimeString(t.Time)
		ageColor := getTafAgeColor(t.Time)

		labelColor.Fprint(&sb, "Issued: ")
		dateColor.Fprint(&sb, t.Time.Format("2006-01-02 15:04 UTC"))
		sb.WriteString(" ")
		ageColor.Fprint(&sb, relTime)
		sb.WriteString("\n")
	}

	// Valid period
	if !t.ValidFrom.IsZero() && !t.ValidTo.IsZero() {
		labelColor.Fprint(&sb, "Valid: ")
		dateColor.Fprint(&sb, t.ValidFrom.Format("2006-01-02 15:04 UTC"))
		sb.WriteString(" to ")
		dateColor.Fprint(&sb, t.ValidTo.Format("2006-01-02 15:04 UTC"))
		sb.WriteString("\n")
	}

	// Forecast periods
	sb.WriteString("\n")
	sectionColor.Fprintln(&sb, "Forecast Periods:")

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

		// Period header with number
		sb.WriteString("\n")
		numberColor.Fprintf(&sb, "%d. ", i+1)
		sb.WriteString(periodType)

		// Time period
		if !forecast.From.IsZero() {
			if forecast.To.IsZero() {
				sb.WriteString(" ")
				dateColor.Fprint(&sb, forecast.From.Format("2006-01-02 15:04 UTC"))
				sb.WriteString(" until end of forecast")
			} else {
				sb.WriteString(" ")
				dateColor.Fprint(&sb, forecast.From.Format("2006-01-02 15:04 UTC"))
				sb.WriteString(" to ")
				dateColor.Fprint(&sb, forecast.To.Format("2006-01-02 15:04 UTC"))
			}
		}
		sb.WriteString("\n")

		// Wind
		windStr := formatWind(forecast.Wind)
		if windStr != "" {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Wind: ")
			sb.WriteString(windStr + "\n")
		}

		// Visibility
		visibilityDesc := formatVisibility(forecast.Visibility)
		if visibilityDesc != "" {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Visibility: ")
			sb.WriteString(visibilityDesc + "\n")
		}

		// Weather
		weatherStr := formatWeather(forecast.Weather)
		if weatherStr != "" {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Weather: ")
			sb.WriteString(capitalizeFirst(weatherStr) + "\n")
		}

		// Clouds
		cloudStr := formatClouds(forecast.Clouds)
		if cloudStr != "" {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Clouds: ")
			sb.WriteString(capitalizeFirst(cloudStr) + "\n")
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

// minutesSince calculates the number of minutes elapsed since the given time
func minutesSince(t time.Time) int {
	return int(time.Since(t).Minutes())
}
