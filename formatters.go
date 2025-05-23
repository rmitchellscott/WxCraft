package main

import (
	"fmt"
	"sort"
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

	// Handle standard 4-digit meter visibility format (e.g. "5000" for 5000 meters)
	if visRegexNum.MatchString(visibility) {
		meters, _ := strconv.Atoi(visibility)
		// Special case for visibility less than 50m reported as "0000"
		if meters == 0 {
			return "Less than 50 meters"
		}
		// Special case for 9999 which means unlimited visibility
		if meters == 9999 {
			return "Unlimited visibility (greater than 10 kilometers)"
		}
		return formatNumberWithCommas(meters) + " meters"
	}

	// Handle visibility with direction (e.g. "4000NE")
	matches := visRegexDir.FindStringSubmatch(visibility)
	if matches != nil {
		meters := matches[1]
		direction := matches[2]

		// Special case for 9999 which means unlimited visibility
		if meters == "9999" {
			return "Unlimited visibility in the " + direction + " direction"
		}

		return meters + " meters in the " + direction + " direction"
	}
	// Handle visibility with NDV (No Directional Variation)
	if ndvRegex.MatchString(visibility) {
		matches := ndvRegex.FindStringSubmatch(visibility)
		if len(matches) > 1 {
			visValue := matches[1]
			meters, _ := strconv.Atoi(visValue)

			// Special case for 0000 - less than 50 meters
			if meters == 0 {
				return "Less than 50 meters in all directions"
			}

			// Special case for 9999 - unlimited visibility
			if meters == 9999 {
				return "Unlimited visibility in all directions"
			}

			return fmt.Sprintf("%s meters in all directions",
				formatNumberWithCommas(meters))
		}
	}
	return visibility
}

// formatWind converts a Wind struct to a human-readable string
func formatWind(wind Wind) string {
	// if wind.Speed == 0 && wind.Direction == "" {
	// 	return ""
	// }

	if wind.Speed == nil && wind.Gust == 0 {
		return ""
	}

	windStr := ""
	if wind.Direction == "VRB" {
		windStr += "Variable"
	} else if wind.Direction != "" && wind.Direction != "0" {
		windStr += fmt.Sprintf("From %s°", wind.Direction)
	}

	unitLabel := "knots"
	if wind.Unit == "MPS" {
		unitLabel = "meters per second"
	}

	if wind.Speed != nil && *wind.Speed > 0 {
		windStr += fmt.Sprintf(" at %d %s", *wind.Speed, unitLabel)
		if wind.Gust > 0 {
			windStr += fmt.Sprintf(", gusting to %d %s", wind.Gust, unitLabel)
		}
	} else {
		windStr += fmt.Sprintf(" %d %s", *wind.Speed, unitLabel)
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

// // formatWeather converts a slice of weather strings to a human-readable format
// func formatWeather(weather []string) string {
// 	if len(weather) == 0 {
// 		return ""
// 	}

// 	var weatherStrs []string
// 	for _, wx := range weather {
// 		if desc, ok := weatherDescriptions[wx]; ok {
// 			weatherStrs = append(weatherStrs, desc)
// 		} else {
// 			weatherStrs = append(weatherStrs, wx)
// 		}
// 	}

//		return strings.Join(weatherStrs, ", ")
//	}
//
// WeatherCode represents a weather code and its properties
type WeatherCode struct {
	Description string
	Position    int
}

// formatWeather converts weather code strings into human-readable descriptions
func formatWeather(weather []string) string {
	if len(weather) == 0 {
		return ""
	}

	var formattedWeather []string

	for _, wxCode := range weather {
		// Split the weather code by spaces to handle multiple elements
		elements := strings.Fields(wxCode)

		if len(elements) == 1 {
			// Single code with no spaces (like "VCHZ")
			formattedWeather = append(formattedWeather, formatWeatherElement(wxCode))
		} else {
			// Multiple codes separated by spaces
			var elementDescriptions []string

			for _, element := range elements {
				elementDescriptions = append(elementDescriptions, formatWeatherElement(element))
			}

			formattedWeather = append(formattedWeather, strings.Join(elementDescriptions, ", "))
		}
	}

	return strings.Join(formattedWeather, ", ")
}

// formatWeatherElement handles a single weather element, with or without combined codes
func formatWeatherElement(code string) string {
	// First check if this is a simple code we already know
	if wc, ok := weatherCodes[code]; ok {
		return wc.Description
	}

	// If not a simple code, try to break it down into components
	type ParsedPart struct {
		Description string
		Position    int
	}

	var parts []ParsedPart
	remainingCode := code

	// Process the code by looking for known two-letter and one-letter codes
	for len(remainingCode) > 0 {
		found := false

		// Try to match 2-letter codes first
		if len(remainingCode) >= 2 {
			twoLetters := remainingCode[:2]
			if wc, ok := weatherCodes[twoLetters]; ok {
				found = true
				parts = append(parts, ParsedPart{
					Description: wc.Description,
					Position:    wc.Position,
				})
				remainingCode = remainingCode[2:]
				continue
			}
		}

		// If no 2-letter code matched, try 1-letter codes (like "+" or "-")
		if len(remainingCode) >= 1 {
			oneLetter := remainingCode[:1]
			if wc, ok := weatherCodes[oneLetter]; ok {
				found = true
				parts = append(parts, ParsedPart{
					Description: wc.Description,
					Position:    wc.Position,
				})
				remainingCode = remainingCode[1:]
				continue
			}
		}

		// If we didn't find a match, we can't completely parse this code
		if !found {
			// If we parsed at least part of the code, add the remaining as a main phenomenon
			if len(parts) > 0 {
				parts = append(parts, ParsedPart{
					Description: remainingCode,
					Position:    1, // Treat unparsed remainder as main phenomenon
				})
			} else {
				// If we couldn't parse anything, return the original code
				return code
			}
			break
		}
	}

	// If we parsed the whole code but didn't find any main phenomenon (position 1),
	// promote the first modifier to main phenomenon if available
	hasMainPhenomenon := false
	for _, part := range parts {
		if part.Position == 1 {
			hasMainPhenomenon = true
			break
		}
	}

	if !hasMainPhenomenon && len(parts) > 0 {
		// Look for modifiers to promote
		for i, part := range parts {
			if part.Position == 2 { // Modifier
				parts[i].Position = 1 // Promote to main phenomenon
				hasMainPhenomenon = true
				break
			}
		}

		// If still no main phenomenon, promote the last prefix
		if !hasMainPhenomenon {
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i].Position == 0 { // Prefix
					parts[i].Position = 1 // Promote to main phenomenon
					break
				}
			}
		}
	}

	// Sort parts by position
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Position < parts[j].Position
	})

	// Build the final description
	var descriptions []string
	for _, part := range parts {
		descriptions = append(descriptions, part.Description)
	}

	// If we couldn't parse anything meaningful, return the original code
	if len(descriptions) == 0 {
		return code
	}

	return strings.Join(descriptions, " ")
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
			// Split the variation at the 'V' character
			parts := strings.Split(m.WindVariation, "V")
			if len(parts) == 2 {
				sb.WriteString(fmt.Sprintf(" (varying between %s° and %s°)", parts[0], parts[1]))
			} else {
				// Fallback in case the format is unexpected
				sb.WriteString(" (varying between " + m.WindVariation + ")")
			}
		}
		sb.WriteString("\n")
	}

	// Visibility
	visibilityDesc := formatVisibility(m.Visibility)
	if visibilityDesc != "" {
		labelColor.Fprint(&sb, "Visibility: ")
		sb.WriteString(visibilityDesc + "\n")
	}

	// Vertical visibility - show if available
	if m.VertVis > 0 {
		labelColor.Fprint(&sb, "Vertical Visibility: ")
		sb.WriteString(fmt.Sprintf("%s feet\n", formatNumberWithCommas(m.VertVis*100)))
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
	if m.Temperature == nil {
		// Case for missing temperature
		labelColor.Fprint(&sb, "Temperature: ")
		sb.WriteString("Not available\n")
	} else {
		tempF := CelsiusToFahrenheit(*m.Temperature)
		labelColor.Fprint(&sb, "Temperature: ")
		sb.WriteString(fmt.Sprintf("%d°C | %d°F\n", *m.Temperature, tempF))
	}

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

	// Wind Shear
	if len(m.WindShear) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Wind Shear:")
		for _, ws := range m.WindShear {
			if ws.Type == "RWY" {
				if ws.Runway != "" {
					sb.WriteString(fmt.Sprintf("  Windshear on runway %s\n", ws.Runway))
				} else if ws.Phase == "TKOF" {
					sb.WriteString("  Takeoff windshear\n")
				} else if ws.Phase == "LDG" {
					sb.WriteString("  Landing windshear\n")
				} else if ws.Phase == "ALL" {
					sb.WriteString("  All runways\n")
				} else {
					sb.WriteString("  Runway windshear\n") // Fallback
				}
			} else if ws.Type == "ALT" {
				var directionStr string
				if ws.Wind.Direction == "VRB" {
					directionStr = "Variable"
				} else {
					directionStr = fmt.Sprintf("From %s°", ws.Wind.Direction)
				}

				sb.WriteString(fmt.Sprintf("  At %d feet: %s at %d %s\n",
					ws.Altitude*100,
					directionStr,
					ws.Wind.Speed,
					ws.Wind.Unit))
			}
		}
	}

	// Runway Conditions and Visual Range
	if len(m.RunwayConditions) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Runway Conditions:")
		for _, cond := range m.RunwayConditions {
			// Format the runway number
			sb.WriteString("  Runway " + cond.Runway + ": ")

			// Handle cleared runways
			if cond.Cleared {
				sb.WriteString(fmt.Sprintf("Cleared of deposits %d minutes ago", cond.ClearedTime))
				sb.WriteString("\n")
				continue
			}

			// Handle variable visibility
			if cond.VisMax > 0 {
				// Variables for readability
				minPrefix := ""
				if cond.Prefix == "M" {
					minPrefix = "less than "
				} else if cond.Prefix == "P" {
					minPrefix = "more than "
				}

				// Handle max value prefix (if any)
				maxPrefix := ""

				// Format unit
				unit := "meters"
				if cond.Unit == "FT" {
					unit = "feet"
				}

				sb.WriteString(fmt.Sprintf("Visibility between %s%d and %s%d %s",
					minPrefix, cond.VisMin, maxPrefix, cond.VisMax, unit))
			} else {
				// Non-variable visibility
				prefix := ""
				if cond.Prefix == "M" {
					prefix = "Less than "
				} else if cond.Prefix == "P" {
					prefix = "More than "
				}

				unit := "meters"
				if cond.Unit == "FT" {
					unit = "feet"
				}

				sb.WriteString(fmt.Sprintf("%s%d %s", prefix, cond.Visibility, unit))
			}

			// Add trend if available
			if cond.Trend != "" {
				trendMap := map[string]string{
					"D": " (decreasing)",
					"U": " (increasing)",
					"N": " (no change)",
				}
				if desc, ok := trendMap[cond.Trend]; ok {
					sb.WriteString(desc)
				} else {
					// Fallback for unrecognized trend
					sb.WriteString(fmt.Sprintf(" (trend: %s)", cond.Trend))
				}
			}

			sb.WriteString("\n")
		}
	} else if len(m.RVR) > 0 {
		// Legacy RVR display (only used if no RunwayConditions are available)
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

		// Vertical visibility
		if forecast.VertVis > 0 {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Vertical Visibility: ")
			sb.WriteString(fmt.Sprintf("%s feet\n", formatNumberWithCommas(forecast.VertVis*100)))
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

		// Wind Shear
		if len(forecast.WindShear) > 0 {
			sb.WriteString("   ")
			labelColor.Fprint(&sb, "Wind Shear: ")
			for i, ws := range forecast.WindShear {
				if i > 0 {
					sb.WriteString("   ")
				}
				if ws.Type == "RWY" {
					if ws.Runway != "" {
						sb.WriteString(fmt.Sprintf("%s runway %s", ws.Phase, ws.Runway))
					} else {
						sb.WriteString(fmt.Sprintf("%s all runways", ws.Phase))
					}
				} else if ws.Type == "ALT" {
					var directionStr string
					if ws.Wind.Direction == "VRB" {
						directionStr = "Variable"
					} else {
						directionStr = fmt.Sprintf("From %s°", ws.Wind.Direction)
					}

					sb.WriteString(fmt.Sprintf("At %d feet: %s at %d %s",
						ws.Altitude*100,
						directionStr,
						ws.Wind.Speed,
						ws.Wind.Unit))
				}
				sb.WriteString("\n")
			}
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
