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
	sectionColor    = color.New(color.FgBlue)
	numberColor     = color.New(color.FgGreen)
	remarkCodeColor = color.New(color.FgGreen)
	functionColor   = color.New(color.FgMagenta)

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

	// CAVOK (Ceiling And Visibility OK)
	if visibility == "CAVOK" {
		return "Greater than 10 km"
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

	// Handle meter-based visibility
	if strings.HasSuffix(visibility, "M") {
		// Added M suffix to indicate meters
		meters := visibility[:len(visibility)-1]
		return meters + " meters"
	}

	// Handle visibility with direction (e.g. "4000NE")
	matches := visRegexDir.FindStringSubmatch(visibility)
	if matches != nil {
		meters := matches[1]
		direction := matches[2]
		return meters + " meters in the " + direction + " direction"
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

	unitLabel := "knots"
	if wind.Unit == "MPS" {
		unitLabel = "meters per second"
	}

	if wind.Speed > 0 {
		windStr += fmt.Sprintf(" at %d %s", wind.Speed, unitLabel)
		if wind.Gust > 0 {
			windStr += fmt.Sprintf(", gusting to %d %s", wind.Gust, unitLabel)
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

// formatSpecialCodes converts special codes to human-readable format
func formatSpecialCodes(codes []string) string {
	if len(codes) == 0 {
		return ""
	}

	var descriptions []string
	for _, code := range codes {
		if desc, ok := specialConditions[code]; ok {
			descriptions = append(descriptions, desc)
		} else {
			descriptions = append(descriptions, code)
		}
	}

	return strings.Join(descriptions, ", ")
}

// getMetarAgeColor returns the appropriate color based on METAR age
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
	sb.WriteString(m.Station)

	// Add site info if available
	if m.SiteInfo.Name != "" && m.SiteInfo.Name != m.Station {
		sb.WriteString(" (")
		siteInfo := formatSiteInfo(m.SiteInfo)
		sb.WriteString(siteInfo)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

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
		sb.WriteString(windStr)

		// Add wind variation if available
		if m.WindVariation != "" {
			sb.WriteString(" (varying between " + m.WindVariation + ")")
		}

		sb.WriteString("\n")
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
	if m.DewPoint == nil {
		// Case for missing dew point
		labelColor.Fprint(&sb, "Dew Point: ")
		sb.WriteString("Not available\n")
	} else {
		dewPointF := CelsiusToFahrenheit(*m.DewPoint)
		labelColor.Fprint(&sb, "Dew Point: ")
		sb.WriteString(fmt.Sprintf("%d°C | %d°F\n", *m.DewPoint, dewPointF))
	}

	// Pressure with conversion to opposite unit
	if m.Pressure > 0 {
		labelColor.Fprint(&sb, "Pressure: ")
		if m.PressureUnit == "inHg" {
			// Convert inHg to hPa/millibars
			pressureHpa := InHgToMillibars(m.Pressure)
			sb.WriteString(fmt.Sprintf("%.2f inHg | %.1f hPa\n", m.Pressure, pressureHpa))
		} else if m.PressureUnit == "hPa" {
			// Convert hPa/millibars to inHg
			pressureInHg := m.Pressure / 33.8639
			sb.WriteString(fmt.Sprintf("%.1f hPa | %.2f inHg\n", m.Pressure, pressureInHg))
		} else {
			// If no unit is specified, default to inHg with hPa conversion
			pressureHpa := InHgToMillibars(m.Pressure)
			sb.WriteString(fmt.Sprintf("%.2f inHg | %.1f hPa\n", m.Pressure, pressureHpa))
		}
	}

	// Runway Visual Range (RVR)
	if len(m.RVR) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Runway Visual Range:")
		for _, rvr := range m.RVR {
			matches := rvrRegex.FindStringSubmatch(rvr)
			if matches != nil {
				runway := matches[1]
				visibility := matches[2]
				trend := matches[3]

				// Format the runway number
				sb.WriteString("  Runway " + runway + ": ")

				// Format visibility with indicators
				if strings.HasPrefix(visibility, "P") {
					sb.WriteString("More than " + visibility[1:] + " meters")
				} else if strings.HasPrefix(visibility, "M") {
					sb.WriteString("Less than " + visibility[1:] + " meters")
				} else {
					sb.WriteString(visibility + " meters")
				}

				// Add trend if available
				if trend != "" {
					trendMap := map[string]string{
						"D": " (decreasing)",
						"U": " (increasing)",
						"N": " (no change)",
					}
					if desc, ok := trendMap[trend]; ok {
						sb.WriteString(desc)
					}
				}

				sb.WriteString("\n")
			} else {
				// Fallback for unmatched RVR format
				sb.WriteString("  " + rvr + "\n")
			}
		}
	}

	// Special codes
	if len(m.SpecialCodes) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Special Conditions:")
		for _, code := range m.SpecialCodes {
			desc := code
			if val, ok := specialConditions[code]; ok {
				desc = val
			}

			// Add bullet and capitalize first letter
			sb.WriteString("  • ")
			sb.WriteString(capitalizeFirst(desc) + "\n")
		}
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

// Helper function to format site information
func formatSiteInfo(info SiteInfo) string {
	parts := []string{}

	if info.Name != "" {
		parts = append(parts, info.Name)
	}

	if info.State != "" {
		parts = append(parts, info.State)
	}

	if info.Country != "" {
		parts = append(parts, info.Country)
	}

	return strings.Join(parts, ", ")
}

// FormatTAF formats a TAF struct for display with colors
func FormatTAF(t TAF) string {
	var sb strings.Builder

	// Station
	labelColor.Fprint(&sb, "Station: ")
	sb.WriteString(t.Station)

	// Add site info if available
	if t.SiteInfo.Name != "" && t.SiteInfo.Name != t.Station {
		sb.WriteString(" (")
		siteInfo := formatSiteInfo(t.SiteInfo)
		sb.WriteString(siteInfo)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

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
		switch {
		case forecast.Type == "BASE":
			periodType = "Base Forecast"
		case forecast.Type == "FM":
			periodType = "From"
		case forecast.Type == "TEMPO":
			periodType = "Temporary"
		case forecast.Type == "BECMG":
			periodType = "Becoming"
		case strings.HasPrefix(forecast.Type, "PROB"):
			// Handle PROB forecasts with the probability value
			periodType = fmt.Sprintf("%d%% Probability", forecast.Probability)
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

// FormatSiteInfo returns a formatted string with the site information
func (m METAR) FormatSiteInfo() string {
	parts := []string{}

	if m.SiteInfo.Name != "" {
		parts = append(parts, m.SiteInfo.Name)
	}

	if m.SiteInfo.State != "" {
		parts = append(parts, m.SiteInfo.State)
	}

	if m.SiteInfo.Country != "" {
		parts = append(parts, m.SiteInfo.Country)
	}

	if len(parts) == 0 {
		return m.Station // Fallback to station code if no site info
	}

	return strings.Join(parts, ", ")
}

func (t TAF) FormatSiteInfo() string {
	parts := []string{}

	if t.SiteInfo.Name != "" {
		parts = append(parts, t.SiteInfo.Name)
	}

	if t.SiteInfo.State != "" {
		parts = append(parts, t.SiteInfo.State)
	}

	if t.SiteInfo.Country != "" {
		parts = append(parts, t.SiteInfo.Country)
	}

	if len(parts) == 0 {
		return t.Station // Fallback to station code if no site info
	}

	return strings.Join(parts, ", ")
}
