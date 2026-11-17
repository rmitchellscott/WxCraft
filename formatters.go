package main

import (
	"fmt"
	"math"
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

	if visibility == "////" || visibility == "////SM" {
		return "Not reported"
	}

	// CAVOK (Ceiling And Visibility OK)
	if visibility == "CAVOK" {
		return "Greater than 10 km"
	}

	if matches := visRegexKM.FindStringSubmatch(visibility); matches != nil {
		return matches[1] + " kilometers"
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

		metersValue, _ := strconv.Atoi(meters)
		return formatNumberWithCommas(metersValue) + " meters in the " + direction + " direction"
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
	if wind.Direction == "" && wind.Speed == nil {
		return ""
	}

	var direction string
	switch wind.Direction {
	case "VRB":
		direction = "Variable"
	case "///":
		direction = "Direction not reported"
	case "", "0":
	default:
		direction = fmt.Sprintf("From %s°", wind.Direction)
	}

	if wind.Speed == nil {
		if wind.Direction == "///" {
			return "Not reported"
		}
		return direction + ", speed not reported"
	}

	if *wind.Speed == 0 && wind.Gust == 0 {
		return "Calm"
	}

	unitLabel := "knots"
	if wind.Unit == "MPS" {
		unitLabel = "meters per second"
	}

	windStr := fmt.Sprintf("%d %s", *wind.Speed, unitLabel)
	if direction != "" {
		windStr = direction + " at " + windStr
	}
	if wind.Gust > 0 {
		windStr += fmt.Sprintf(", gusting to %d %s", wind.Gust, unitLabel)
	}

	return windStr
}

// formatAltitudeWindShear describes a WShhh/dddffKT wind shear group
func formatAltitudeWindShear(ws WindShear) string {
	return fmt.Sprintf("At %s feet: %s", formatNumberWithCommas(ws.Altitude*100), formatWind(ws.Wind))
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

		typeDesc := cloud.Type
		if t, ok := cloudTypes[cloud.Type]; ok {
			typeDesc = t
		}

		var notReported []string
		coverageMissing := cloud.Coverage == "///"
		if coverageMissing {
			coverStr = "clouds"
			if cloud.Type != "" && cloud.Type != "///" {
				coverStr = typeDesc
			}
			notReported = append(notReported, "coverage")
		}

		cloudDesc := coverStr
		isClear := cloud.Coverage == "CLR" || cloud.Coverage == "SKC"
		switch {
		case cloud.HeightMissing:
			notReported = append(notReported, "height")
		case cloud.Height > 0:
			cloudDesc = fmt.Sprintf("%s at %s feet", coverStr, formatNumberWithCommas(cloud.Height))
		case !isClear:
			cloudDesc = coverStr + " below 100 feet"
		}

		if cloud.Type == "///" {
			notReported = append(notReported, "type")
		} else if cloud.Type != "" && !coverageMissing {
			cloudDesc = fmt.Sprintf("%s (%s)", cloudDesc, typeDesc)
		}

		if len(notReported) > 0 {
			cloudDesc = fmt.Sprintf("%s (%s not reported)", cloudDesc, joinWithAnd(notReported))
		}

		cloudStrs = append(cloudStrs, cloudDesc)
	}

	return strings.Join(cloudStrs, ", ")
}

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
	switch code {
	case "//":
		return "not reported"
	case "RE//":
		return "recent weather not reported"
	}

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
		descriptions = append(descriptions, describeSpecialCode(code))
	}

	return strings.Join(descriptions, ", ")
}

func describeSpecialCode(code string) string {
	if desc, ok := specialConditions[code]; ok {
		return desc
	}

	if matches := militaryColorRegex.FindStringSubmatch(code); matches != nil {
		desc := "military color state " + militaryColorStates[matches[2]]
		if matches[1] == "BLACK" {
			desc += ", airfield unusable for non-weather reasons"
		}
		return fmt.Sprintf("%s (%s)", desc, code)
	}

	return code
}

func formatRunwayState(state RunwayState) string {
	var label string
	switch state.Runway {
	case "":
		label = "Aerodrome"
	case "88":
		label = "All runways"
	case "99":
		label = "Runway (last report repeated)"
	default:
		label = "Runway " + state.Runway
	}

	if state.SnowClosed {
		return label + ": Closed due to snow"
	}

	var details []string
	if state.Cleared {
		details = append(details, "contamination cleared")
		if state.Friction != "//" {
			details = append(details, describeRunwayFriction(state.Friction))
		}
		return label + ": " + capitalizeFirst(strings.Join(details, ", "))
	}

	if strings.Trim(state.Deposit+state.Extent+state.Depth+state.Friction, "/") == "" {
		return label + ": Not reported"
	}

	if desc, ok := runwayDeposits[state.Deposit]; ok {
		details = append(details, desc)
	} else {
		details = append(details, "deposit not reported")
	}

	if desc, ok := runwayContaminationExtents[state.Extent]; ok {
		details = append(details, desc)
	} else {
		details = append(details, "extent not reported")
	}

	details = append(details, describeRunwayDepositDepth(state.Depth), describeRunwayFriction(state.Friction))

	return label + ": " + capitalizeFirst(strings.Join(details, ", "))
}

// describeRunwayDepositDepth decodes the eReR depth digits of a runway state group (WMO code table 1079)
func describeRunwayDepositDepth(depth string) string {
	value, err := strconv.Atoi(depth)
	switch {
	case err != nil:
		return "depth not significant or not measurable"
	case value == 0:
		return "depth less than 1 mm"
	case value <= 90:
		return fmt.Sprintf("depth %d mm", value)
	case value >= 92 && value <= 97:
		return fmt.Sprintf("depth %d cm", (value-90)*5)
	case value == 98:
		return "depth 40 cm or more"
	case value == 99:
		return "runway not operational"
	}
	return "depth code " + depth
}

// describeRunwayFriction decodes the BRBR friction digits of a runway state group (WMO code table 0366)
func describeRunwayFriction(friction string) string {
	brakingActions := map[int]string{
		91: "braking action poor",
		92: "braking action medium to poor",
		93: "braking action medium",
		94: "braking action medium to good",
		95: "braking action good",
		99: "braking action unreliable",
	}

	value, err := strconv.Atoi(friction)
	switch {
	case err != nil:
		return "braking action not reported"
	case value <= 90:
		return fmt.Sprintf("friction coefficient 0.%02d", value)
	}
	if desc, ok := brakingActions[value]; ok {
		return desc
	}
	return "friction code " + friction
}

func formatSeaState(sea SeaState) string {
	var details []string

	if sea.Temperature != nil {
		details = append(details, fmt.Sprintf("surface temperature %d°C | %d°F", *sea.Temperature, CelsiusToFahrenheit(*sea.Temperature)))
	} else {
		details = append(details, "surface temperature not reported")
	}

	switch {
	case sea.WaveHeight != nil:
		details = append(details, fmt.Sprintf("significant wave height %.1f m", float64(*sea.WaveHeight)/10))
	case sea.State != "":
		details = append(details, "sea state "+seaStates[sea.State])
	default:
		details = append(details, "sea state not reported")
	}

	return capitalizeFirst(strings.Join(details, ", "))
}

// joinWithAnd joins items as "a", "a and b", or "a, b and c"
func joinWithAnd(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
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

	if m.ReportType != "" {
		labelColor.Fprint(&sb, "Report: ")
		sb.WriteString(reportTypes[m.ReportType] + "\n")
	}

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
		sb.WriteString(windStr + formatWindVariation(m.WindVariation) + "\n")
	}

	// Visibility
	visibilityDesc := formatVisibility(m.Visibility)
	if visibilityDesc != "" {
		labelColor.Fprint(&sb, "Visibility: ")
		sb.WriteString(visibilityDesc + "\n")
	}

	if m.MinVisibility != "" {
		labelColor.Fprint(&sb, "Minimum Visibility: ")
		sb.WriteString(formatVisibility(m.MinVisibility) + "\n")
	}

	derived := deriveMETAR(m)

	// Vertical visibility - show if available
	if m.VertVis != nil {
		labelColor.Fprint(&sb, "Vertical Visibility: ")
		sb.WriteString(formatVerticalVisibility(*m.VertVis) + "\n")
	} else if m.VertVisMissing {
		labelColor.Fprint(&sb, "Vertical Visibility: ")
		sb.WriteString("Not reported\n")
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

	if derived.ceilingKnown {
		labelColor.Fprint(&sb, "Ceiling: ")
		sb.WriteString(formatCeiling(derived.ceiling) + "\n")
	}
	if derived.flightCategory != "" {
		labelColor.Fprint(&sb, "Flight Category: ")
		sb.WriteString(derived.flightCategory + "\n")
	}

	labelColor.Fprint(&sb, "Temperature: ")
	sb.WriteString(formatCelsius(derived.temperature, derived.tenths) + "\n")
	labelColor.Fprint(&sb, "Dew Point: ")
	sb.WriteString(formatCelsius(derived.dewPoint, derived.tenths) + "\n")

	if derived.relativeHumidity != nil {
		labelColor.Fprint(&sb, "Relative Humidity: ")
		sb.WriteString(fmt.Sprintf("%.0f%%\n", *derived.relativeHumidity))
	}

	if m.Pressure > 0 || m.PressureUnit != "" {
		labelColor.Fprint(&sb, "Pressure: ")
		sb.WriteString(formatPressure(m.Pressure, m.PressureUnit) + "\n")
	}

	if m.SecondPressure.Unit != "" {
		labelColor.Fprint(&sb, "Second Pressure Group: ")
		sb.WriteString(formatPressure(m.SecondPressure.Value, m.SecondPressure.Unit) + "\n")
	}

	if derived.densityAltitude != nil {
		labelColor.Fprint(&sb, "Density Altitude: ")
		sb.WriteString(formatNumberWithCommas(*derived.densityAltitude) + " feet\n")
	}

	if m.StationPressure > 0 {
		labelColor.Fprint(&sb, "Station Pressure: ")
		sb.WriteString(fmt.Sprintf("%.1f hPa\n", m.StationPressure))
	}

	for _, sea := range m.SeaStates {
		labelColor.Fprint(&sb, "Sea: ")
		sb.WriteString(formatSeaState(sea) + "\n")
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
				sb.WriteString("  " + formatAltitudeWindShear(ws) + "\n")
			}
		}
	}

	if len(m.RunwayConditions) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Runway Visual Range:")
		for _, cond := range m.RunwayConditions {
			// Format the runway number
			sb.WriteString("  Runway " + cond.Runway + ": ")

			if cond.Missing {
				sb.WriteString("Not reported\n")
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
	}

	if len(m.RunwayStates) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Runway State:")
		for _, state := range m.RunwayStates {
			sb.WriteString("  " + formatRunwayState(state) + "\n")
		}
	}

	if len(m.Trends) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Trends:")
		for i, trend := range m.Trends {
			formatPeriod(&sb, i+1, trend, "onward", nil)
		}
	}

	// Special codes
	if len(m.SpecialCodes) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Special Conditions:")
		for _, code := range m.SpecialCodes {
			desc := describeSpecialCode(code)

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

	var status []string
	for _, flag := range []struct {
		set         bool
		description string
	}{
		{t.Amended, "amended"},
		{t.Corrected, "corrected"},
		{t.Nil, "missing (NIL)"},
		{t.Cancelled, "cancelled"},
	} {
		if flag.set {
			status = append(status, flag.description)
		}
	}
	if len(status) > 0 {
		labelColor.Fprint(&sb, "Report: ")
		sb.WriteString(capitalizeFirst(strings.Join(status, ", ")) + "\n")
	}

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

	if len(t.Forecasts) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Forecast Periods:")
		skies := periodSkies(t.Forecasts)
		for i, forecast := range t.Forecasts {
			formatPeriod(&sb, i+1, forecast, "until end of forecast", &skies[i])
		}
	}

	if len(t.Temperatures) > 0 {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Temperatures:")
		for _, temperature := range t.Temperatures {
			kind := map[string]string{"MAX": "Maximum", "MIN": "Minimum"}[temperature.Kind]
			if kind == "" {
				kind = "Forecast"
			}
			sb.WriteString(fmt.Sprintf("  %s %d°C | %d°F at ", kind, temperature.Celsius, CelsiusToFahrenheit(temperature.Celsius)))
			dateColor.Fprint(&sb, temperature.At.Format("2006-01-02 15:04 UTC"))
			sb.WriteString("\n")
		}
	}

	if t.Remarks != "" {
		sb.WriteString("\n")
		sectionColor.Fprintln(&sb, "Remarks:")
		sb.WriteString("  " + t.Remarks + "\n")
	}

	return sb.String()
}

// formatPeriod writes a numbered TAF forecast period or METAR trend group. openEnded follows the
// start time of a period that has no end time. sky, when not nil, is the visibility and sky in
// force during the period, from which its ceiling and flight category are shown.
func formatPeriod(sb *strings.Builder, number int, forecast Forecast, openEnded string, sky *skyCondition) {
	periodType, ok := periodTypes[forecast.Type]
	if !ok {
		periodType = forecast.Type
	}
	switch {
	case forecast.Type == "PROB":
		periodType = fmt.Sprintf("%d%% Probability", forecast.Probability)
	case forecast.Probability > 0:
		periodType = fmt.Sprintf("%d%% Probability, %s", forecast.Probability, periodType)
	}

	sb.WriteString("\n")
	numberColor.Fprintf(sb, "%d. ", number)
	sb.WriteString(periodType)

	writeTime := func(prefix string, at time.Time) {
		sb.WriteString(prefix)
		dateColor.Fprint(sb, at.Format("2006-01-02 15:04 UTC"))
	}
	switch {
	case !forecast.At.IsZero():
		writeTime(" at ", forecast.At)
	case !forecast.From.IsZero() && !forecast.To.IsZero():
		writeTime(" ", forecast.From)
		writeTime(" to ", forecast.To)
	case !forecast.From.IsZero():
		writeTime(" ", forecast.From)
		sb.WriteString(" " + openEnded)
	case !forecast.To.IsZero():
		writeTime(" until ", forecast.To)
	}
	if forecast.InEffect {
		sb.WriteString(" (in effect)")
	}
	sb.WriteString("\n")

	writeLine := func(label, value string) {
		sb.WriteString("   ")
		labelColor.Fprint(sb, label+": ")
		sb.WriteString(value + "\n")
	}

	if windStr := formatWind(forecast.Wind); windStr != "" {
		writeLine("Wind", windStr+formatWindVariation(forecast.WindVariation))
	}
	if visibilityDesc := formatVisibility(forecast.Visibility); visibilityDesc != "" {
		writeLine("Visibility", visibilityDesc)
	}
	if forecast.MinVisibility != "" {
		writeLine("Minimum Visibility", formatVisibility(forecast.MinVisibility))
	}
	if forecast.VertVis != nil {
		writeLine("Vertical Visibility", formatVerticalVisibility(*forecast.VertVis))
	}

	switch {
	case len(forecast.Weather) > 0:
		writeLine("Weather", capitalizeFirst(formatWeather(forecast.Weather)))
	case forecast.NSW:
		writeLine("Weather", "No significant weather")
	}

	switch {
	case len(forecast.Clouds) > 0:
		writeLine("Clouds", capitalizeFirst(formatClouds(forecast.Clouds)))
	case forecast.NSC:
		writeLine("Clouds", "No significant clouds")
	}

	if sky != nil {
		if ceiling, ok := sky.ceiling(); ok {
			writeLine("Ceiling", formatCeiling(ceiling))
		}
		if category := sky.flightCategory(); category != "" {
			writeLine("Flight Category", category)
		}
	}

	if forecast.PressureUnit != "" {
		writeLine("Pressure", formatPressure(forecast.Pressure, forecast.PressureUnit))
	}

	if len(forecast.WindShear) > 0 {
		sb.WriteString("   ")
		labelColor.Fprint(sb, "Wind Shear: ")
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
				sb.WriteString(formatAltitudeWindShear(ws))
			}
			sb.WriteString("\n")
		}
	}

	for _, layer := range forecast.Icing {
		writeLine("Icing", capitalizeFirst(formatHazardLayer(layer, icingIntensities)))
	}
	for _, layer := range forecast.Turbulence {
		writeLine("Turbulence", capitalizeFirst(formatHazardLayer(layer, turbulenceIntensities)))
	}
}

// formatWindVariation returns the suffix that follows the wind description for a dddVddd group
func formatWindVariation(variation string) string {
	if variation == "" {
		return ""
	}
	if strings.Contains(variation, "/") {
		return " (variation not reported)"
	}
	parts := strings.Split(variation, "V")
	if len(parts) != 2 {
		return " (varying between " + variation + ")"
	}
	return fmt.Sprintf(" (varying between %s° and %s°)", parts[0], parts[1])
}

// formatPressure shows the reported pressure followed by its conversion; zero means not reported
func formatPressure(pressure float64, unit string) string {
	switch {
	case pressure == 0:
		return "Not available"
	case unit == "hPa":
		return fmt.Sprintf("%.1f hPa | %.2f inHg", pressure, MillibarsToInHg(pressure))
	}
	return fmt.Sprintf("%.2f inHg | %.1f hPa", pressure, InHgToMillibars(pressure))
}

func formatVerticalVisibility(hundredsOfFeet int) string {
	if hundredsOfFeet == 0 {
		return "Below 100 feet"
	}
	return formatNumberWithCommas(hundredsOfFeet*100) + " feet"
}

func formatCeiling(c ceiling) string {
	if c.none {
		return "None"
	}
	if c.feet == 0 {
		return "Below 100 feet"
	}
	return formatNumberWithCommas(c.feet) + " feet"
}

// formatCelsius shows a temperature with its Fahrenheit conversion, to tenths of a degree Celsius
// when it comes from the T remark group
func formatCelsius(celsius *float64, tenths bool) string {
	if celsius == nil {
		return "Not available"
	}
	decimals := 0
	if tenths {
		decimals = 1
	}
	return fmt.Sprintf("%.*f°C | %d°F", decimals, *celsius, int(math.Round(*celsius*9/5+32)))
}

func formatHazardLayer(layer HazardLayer, intensities map[string]string) string {
	description, ok := intensities[layer.Intensity]
	if !ok {
		description = "intensity code " + layer.Intensity
	}
	base := "the surface"
	if layer.BaseFeet > 0 {
		base = formatNumberWithCommas(layer.BaseFeet) + " feet"
	}
	if layer.ThicknessFeet == 0 {
		return fmt.Sprintf("%s from %s", description, base)
	}
	top := formatNumberWithCommas(layer.BaseFeet + layer.ThicknessFeet)
	return fmt.Sprintf("%s from %s to %s feet", description, base, top)
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
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := strconv.Itoa(n)

	result := ""
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}

	return sign + result
}
