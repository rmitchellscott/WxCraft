package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Color variables for consistent formatting
var (
	errorColor = color.New(color.FgRed).Add(color.Bold)
)

func newRemarkCodes() map[string]string {
	// Common remark codes and their descriptions
	codes := map[string]string{
		"AO1":     "automated station without precipitation sensor",
		"AO2":     "automated station with precipitation sensor",
		"AO1A":    "automated station without precipitation sensor",
		"AO2A":    "automated station with precipitation sensor",
		"SLP":     "sea level pressure",
		"SLPNO":   "sea level pressure information not available",
		"PWINO":   "precipitation identifier sensor not available",
		"PNO":     "precipitation amount not available",
		"FZRANO":  "freezing rain information not available",
		"TSNO":    "thunderstorm information not available",
		"RVRNO":   "runway visual range not available",
		"VISNO":   "visibility at second location not available",
		"CHINO":   "cloud height at second location not available",
		"NOSPECI": "no SPECI reports are taken at this station",
		"FIRST":   "first observation after a break in coverage",
		"LAST":    "last observation before a break in coverage",
		"RMK":     "remarks indicator",
		"PRESRR":  "pressure rising rapidly",
		"PRESFR":  "pressure falling rapidly",
		"NOSIG":   "no significant changes expected",
		"TEMPO":   "temporary",
		"BECMG":   "becoming",
		"VIRGA":   "precipitation not reaching ground",
		"FROPA":   "frontal passage",
		"BINOVC":  "breaks in overcast",
		"SKC":     "sky clear",
		"$":       "weather observing equipment requires maintenance",
	}

	// Fix invalid remark codes starting with "A0"
	for k, v := range codes {
		if strings.HasPrefix(k, "AO") {
			codes["A0"+k[2:]] = v
		}
	}

	return codes
}

type remarkRule struct {
	pattern  *regexp.Regexp
	describe func(match []string) string
}

// remarkPattern anchors a remark group at the start of the remaining remark
// text and requires it to end on a token boundary, so a group can span tokens.
func remarkPattern(expr string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + expr + `)(?: |$)`)
}

// Alternations list longer codes first because Go regexps prefer the first
// alternative that lets the whole pattern match.
const (
	rmkDirection   = `(?:NE|NW|N|SE|SW|S|E|W)`
	rmkDirections  = rmkDirection + `(?:-` + rmkDirection + `)*(?: THRU ` + rmkDirection + `)?`
	rmkPlace       = `(?:(?:DSNT|DIST|VC|OHD|ALQDS|ALQS)(?: ` + rmkDirections + `)?|` + rmkDirections + `)`
	rmkLocation    = rmkPlace + `(?: (?:AND )?` + rmkPlace + `)*`
	rmkRunway      = `RWY ?\d{1,2}[LCR]?`
	rmkVisibility  = `M?\d+ \d/\d{1,2}|M?\d+/\d{1,2}|M?\d+`
	rmkWind        = `(\d{3}|VRB|///)(\d{2,3}|//)(?:G(\d{2,3}))?(KT|MPS)`
	rmkPrecip      = `(?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|PE)`
	rmkWeather     = `(?:MI|PR|BC|DR|BL|SH|TS|FZ)?(?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS)+|VCSH|VCTS|VCFG|TS`
	rmkPhenomenon  = `CBMAM|CB|TCU|ACC|ACSL|SCSL|CCSL|VIRGA|MTNS OBSC|MTN TOPS OBSC|MT OBSC|OBST OBSC|[-+]?(?:` + rmkWeather + `)`
	rmkFrequency   = `OCNL|FRQ|CONS|ISOL`
	rmkCloudLayers = `ACC|ACSL|BLSN|BLDU|TCU|CI|CS|CC|AS|AC|ST|SC|NS|CU|CB|SF|CF|FG|SN|FU|HZ|BR|DZ|RA`
)

var (
	remarkEventRegex         = regexp.MustCompile(`([A-Z]+?)((?:[BE](?:\d{4}|\d{2}))+)`)
	remarkEventTimeRegex     = regexp.MustCompile(`([BE])(\d{4}|\d{2})`)
	remarkEventPhenomenon    = regexp.MustCompile(`^(?:(?:SH|FZ|BL|DR|TS)?` + rmkPrecip + `+|TS|FC|SQ|FG|BR|HZ|FU|VA|DU|SA|PY|SS|DS)$`)
	remarkCloudLayerRegex    = regexp.MustCompile(`(` + rmkCloudLayers + `)(\d)`)
	remarkPlainLanguageRegex = regexp.MustCompile(`^[A-Z][A-Z.,'?/-]*$`)
)

var remarkLocationWords = map[string]string{
	"DSNT":  "distant",
	"DIST":  "distant",
	"VC":    "in the vicinity",
	"OHD":   "overhead",
	"ALQDS": "in all quadrants",
	"ALQS":  "in all quadrants",
	"AND":   "and",
	"THRU":  "through",
	"N":     "north",
	"NE":    "northeast",
	"E":     "east",
	"SE":    "southeast",
	"S":     "south",
	"SW":    "southwest",
	"W":     "west",
	"NW":    "northwest",
	"STNRY": "stationary",
}

var remarkFrequencies = map[string]string{
	"OCNL": "occasional",
	"FRQ":  "frequent",
	"CONS": "continuous",
	"ISOL": "isolated",
}

var remarkLightningTypes = map[string]string{
	"IC": "in-cloud",
	"CC": "cloud-to-cloud",
	"CG": "cloud-to-ground",
	"CA": "cloud-to-air",
}

var remarkCloudNames = map[string]string{
	"CI":    "cirrus",
	"CS":    "cirrostratus",
	"CC":    "cirrocumulus",
	"AS":    "altostratus",
	"AC":    "altocumulus",
	"ACC":   "altocumulus castellanus",
	"ACSL":  "standing lenticular altocumulus",
	"SCSL":  "standing lenticular stratocumulus",
	"CCSL":  "standing lenticular cirrocumulus",
	"ST":    "stratus",
	"SC":    "stratocumulus",
	"NS":    "nimbostratus",
	"CU":    "cumulus",
	"TCU":   "towering cumulus",
	"CB":    "cumulonimbus",
	"CBMAM": "cumulonimbus mammatus",
	"SF":    "stratus fractus",
	"CF":    "cumulus fractus",
	"FG":    "fog",
	"SN":    "snow",
	"BLSN":  "blowing snow",
	"BLDU":  "blowing dust",
	"FU":    "smoke",
	"HZ":    "haze",
	"BR":    "mist",
	"DZ":    "drizzle",
	"RA":    "rain",
	"VIRGA": "virga",

	"MTNS OBSC":     "mountains obscured",
	"MTN TOPS OBSC": "mountain tops obscured",
	"MT OBSC":       "mountains obscured",
	"OBST OBSC":     "obstacles obscured",
}

// WMO code tables 0513, 0515 and 0509, indexed by the code digit.
var (
	lowCloudTypes = []string{
		"none", "cumulus humilis or fractus", "cumulus mediocris or congestus", "cumulonimbus calvus",
		"stratocumulus cumulogenitus", "stratocumulus", "stratus nebulosus or fractus",
		"stratus or cumulus fractus of bad weather", "cumulus and stratocumulus at different levels",
		"cumulonimbus capillatus",
	}
	middleCloudTypes = []string{
		"none", "altostratus translucidus", "altostratus opacus or nimbostratus",
		"altocumulus translucidus at a single level", "patches of altocumulus",
		"altocumulus invading the sky", "altocumulus cumulogenitus",
		"altocumulus with altostratus or nimbostratus", "altocumulus castellanus or floccus",
		"altocumulus of a chaotic sky",
	}
	highCloudTypes = []string{
		"none", "cirrus fibratus or uncinus", "cirrus spissatus, castellanus or floccus",
		"cirrus spissatus cumulonimbogenitus", "cirrus invading the sky",
		"cirrus and cirrostratus below 45 degrees", "cirrus and cirrostratus above 45 degrees",
		"cirrostratus covering the whole sky", "cirrostratus not covering the whole sky", "cirrocumulus",
	}
)

var pressureTendencies = map[string]string{
	"0": "increasing, then decreasing",
	"1": "increasing, then steady",
	"2": "increasing steadily",
	"3": "increasing, then increasing more rapidly",
	"4": "steady",
	"5": "decreasing, then increasing",
	"6": "decreasing, then steady",
	"7": "decreasing steadily",
	"8": "decreasing, then decreasing more rapidly",
}

var remarkRules = []remarkRule{
	{remarkPattern(`A(\d{4})`), func(m []string) string {
		return fmt.Sprintf("altimeter setting %.2f inHg", float64(atoi(m[1]))/100)
	}},
	{remarkPattern(`A(\d{2})\.(\d{2})`), func(m []string) string {
		return fmt.Sprintf("altimeter setting %s.%s inHg", m[1], m[2])
	}},
	{remarkPattern(`QFE(\d{3}\.\d) (\d{1,3})`), func(m []string) string {
		return fmt.Sprintf("QFE %s mmHg, relative humidity %d%%", m[1], atoi(m[2]))
	}},
	{remarkPattern(`QFE(\d{3}(?:\.\d)?)(?:/(\d{3,4}))?`), func(m []string) string {
		description := "QFE " + m[1] + " mmHg"
		if m[2] != "" {
			description += fmt.Sprintf(" (%d hPa)", atoi(m[2]))
		}
		return description
	}},
	{remarkPattern(`QFE(\d{4})`), func(m []string) string {
		return fmt.Sprintf("QFE %d hPa", atoi(m[1]))
	}},
	{remarkPattern(`QBB(\d{3})`), func(m []string) string {
		return fmt.Sprintf("cloud base %d meters", atoi(m[1]))
	}},
	{remarkPattern(`WIND (\d+)FT ` + rmkWind), func(m []string) string {
		return fmt.Sprintf("wind at %d feet %s", atoi(m[1]), describeRemarkWind(m[2:6]))
	}},
	{remarkPattern(`WIND THR(\d{2}[LCR]?) ?` + rmkWind), func(m []string) string {
		return fmt.Sprintf("wind at runway %s threshold %s", m[1], describeRemarkWind(m[2:6]))
	}},
	{remarkPattern(`(` + rmkRunway + `|ARP/|MID|END R\d{2}[LCR]?/|R\d{2}[LCR]?/|WIND ` + rmkRunway + `|WIND [A-Z]+|[A-Z]+ WIND) ?` + rmkWind + `(?: (\d{3})V(\d{3}))?`), func(m []string) string {
		description := describeWindPlace(m[1]) + " wind " + describeRemarkWind(m[2:6])
		if m[6] != "" {
			description += fmt.Sprintf(", varying between %s° and %s°", m[6], m[7])
		}
		return description
	}},
	{remarkPattern(`VIS MIN (\d{4})(` + rmkDirection + `)?`), func(m []string) string {
		description := fmt.Sprintf("minimum visibility %d meters", atoi(m[1]))
		if m[2] != "" {
			description += " to the " + remarkLocationWords[m[2]]
		}
		return description
	}},
	{remarkPattern(`(FEW|SCT|BKN|OVC)(\d{3})? V (FEW|SCT|BKN|OVC)`), func(m []string) string {
		description := "variable sky condition: " + cloudCoverage[m[1]]
		if m[2] != "" {
			description += fmt.Sprintf(" at %d feet", atoi(m[2])*100)
		}
		return description + " varying with " + cloudCoverage[m[3]]
	}},
	{remarkPattern(`((?:WND|WIND|ALSTG|ALTSG|SLP)(?:[ /](?:DATA|WND|ALSTG|ALTSG|SLP))*) (?:ESTMD|ESTD|EST)`), func(m []string) string {
		return describeEstimated(m[1]) + " estimated"
	}},
	{remarkPattern(`(BLACK)?(BLU|WHT|GRN|YLO[12]?|AMB|RED)|BLACK`), describeColorState},
	{remarkPattern(`BIRD HAZARD(?: RWY ?(\d{2}[LCR]?/\d{2}[LCR]?))?`), func(m []string) string {
		if m[1] == "" {
			return "bird hazard"
		}
		return "bird hazard on runway " + m[1]
	}},
	{remarkPattern(`DENSITY ALT (-?\d+)FT`), func(m []string) string {
		return fmt.Sprintf("density altitude %d feet", atoi(m[1]))
	}},
	{remarkPattern(`PCPN (\d+(?:\.\d+)?)MM PAST HR`), func(m []string) string {
		return fmt.Sprintf("%s mm of precipitation in the past hour", m[1])
	}},
	{remarkPattern(`SOG (\d{1,3})`), func(m []string) string {
		return fmt.Sprintf("snow on ground: %d cm", atoi(m[1]))
	}},
	{remarkPattern(`VIS VRB (` + rmkVisibility + `)-(` + rmkVisibility + `)`), func(m []string) string {
		return fmt.Sprintf("variable visibility from %s to %s", describeStatuteMiles(m[1]), describeStatuteMiles(m[2]))
	}},
	{remarkPattern(`CIG VRB (\d{1,3})-(\d{1,3})`), func(m []string) string {
		return fmt.Sprintf("variable ceiling between %d and %d feet", atoi(m[1])*100, atoi(m[2])*100)
	}},
	{remarkPattern(`(?:` + rmkCloudLayers + `)\d(?:(?:` + rmkCloudLayers + `)\d)*`), describeCloudLayers},
	{remarkPattern(`(` + rmkCloudLayers + `) TR`), func(m []string) string {
		return "trace of " + remarkCloudNames[m[1]]
	}},
	{remarkPattern(`(\d)(` + rmkCloudLayers + `)(\d{3})?`), func(m []string) string {
		description := fmt.Sprintf("%s/8 %s", m[1], remarkCloudNames[m[2]])
		if m[3] != "" {
			description += fmt.Sprintf(" at %d feet", atoi(m[3])*100)
		}
		return description
	}},
	{remarkPattern(`PK WND (\d{3})(\d{2,3})/(\d{2})?(\d{2})`), func(m []string) string {
		return fmt.Sprintf("peak wind %s° at %d knots %s", m[1], atoi(m[2]), describeRemarkTime(m[3], m[4]))
	}},
	{remarkPattern(`WSHFT (\d{2})?(\d{2})( FROPA)?`), func(m []string) string {
		description := "wind shift " + describeRemarkTime(m[1], m[2])
		if m[3] != "" {
			description += " due to frontal passage"
		}
		return description
	}},
	{remarkPattern(`(TWR|SFC) VIS (` + rmkVisibility + `)`), func(m []string) string {
		place := map[string]string{"TWR": "tower", "SFC": "surface"}[m[1]]
		return fmt.Sprintf("%s visibility %s", place, describeStatuteMiles(m[2]))
	}},
	{remarkPattern(`VIS (` + rmkVisibility + `)V(` + rmkVisibility + `)`), func(m []string) string {
		return fmt.Sprintf("variable visibility from %s to %s", describeStatuteMiles(m[1]), describeStatuteMiles(m[2]))
	}},
	{remarkPattern(`VIS (` + rmkVisibility + `) (` + rmkRunway + `|` + rmkDirections + `)`), func(m []string) string {
		return fmt.Sprintf("visibility %s %s", describeStatuteMiles(m[1]), describeRemarkLocation(m[2]))
	}},
	{remarkPattern(`VIS (` + rmkDirections + `) (` + rmkVisibility + `)`), func(m []string) string {
		return fmt.Sprintf("visibility %s %s", describeStatuteMiles(m[2]), describeRemarkLocation(m[1]))
	}},
	{remarkPattern(`(CIG|CHINO|VISNO) (` + rmkRunway + `|` + rmkDirections + `)`), func(m []string) string {
		subject := map[string]string{
			"CIG":   "ceiling",
			"CHINO": "cloud height not available",
			"VISNO": "visibility not available",
		}[m[1]]
		return subject + " " + describeRemarkLocation(m[2])
	}},
	{remarkPattern(`CIG (\d{3})V(\d{3})`), func(m []string) string {
		return fmt.Sprintf("variable ceiling between %d and %d feet", atoi(m[1])*100, atoi(m[2])*100)
	}},
	{remarkPattern(`CIG ?(\d{3})(?: (` + rmkRunway + `|` + rmkDirections + `))?`), func(m []string) string {
		description := fmt.Sprintf("ceiling %d feet", atoi(m[1])*100)
		if m[2] != "" {
			description += " " + describeRemarkLocation(m[2])
		}
		return description
	}},
	{remarkPattern(`SLP(\d{3})`), func(m []string) string {
		tenths := atoi(m[1])
		hpa := 1000 + float64(tenths)/10
		if tenths >= 500 {
			hpa -= 100
		}
		return fmt.Sprintf("sea level pressure %.1f hPa", hpa)
	}},
	{remarkPattern(`SLP///`), func(m []string) string {
		return "sea level pressure not available"
	}},
	{remarkPattern(`T([01])(\d{3})(?:([01])(\d{3})|(////))?`), func(m []string) string {
		description := fmt.Sprintf("temperature %.1f°C", signedTenths(m[1], m[2]))
		switch {
		case m[3] != "":
			description += fmt.Sprintf(", dew point %.1f°C", signedTenths(m[3], m[4]))
		case m[5] != "":
			description += ", dew point not available"
		}
		return description
	}},
	{remarkPattern(`1([01])(\d{3})`), func(m []string) string {
		return fmt.Sprintf("6-hour maximum temperature %.1f°C", signedTenths(m[1], m[2]))
	}},
	{remarkPattern(`2([01])(\d{3})`), func(m []string) string {
		return fmt.Sprintf("6-hour minimum temperature %.1f°C", signedTenths(m[1], m[2]))
	}},
	{remarkPattern(`4([01])(\d{3})([01])(\d{3})`), func(m []string) string {
		maxValue := signedTenths(m[1], m[2])
		minValue := signedTenths(m[3], m[4])
		return fmt.Sprintf("24-hour temperature range: max %.1f°C (%.1f°F), min %.1f°C (%.1f°F)",
			maxValue, maxValue*9/5+32, minValue, minValue*9/5+32)
	}},
	{remarkPattern(`5([0-8])(\d{3})`), func(m []string) string {
		return fmt.Sprintf("pressure tendency: %s, %.1f hPa change", pressureTendencies[m[1]], float64(atoi(m[2]))/10)
	}},
	{remarkPattern(`5////`), func(m []string) string {
		return "pressure tendency not available"
	}},
	{remarkPattern(`6(\d{4})`), func(m []string) string {
		return describePrecipitation("3- or 6-hour precipitation", m[1])
	}},
	{remarkPattern(`([67])////`), func(m []string) string {
		period := map[string]string{"6": "3- or 6-hour", "7": "24-hour"}[m[1]]
		return period + " precipitation: indeterminate amount"
	}},
	{remarkPattern(`7(\d{4})`), func(m []string) string {
		return describePrecipitation("24-hour precipitation", m[1])
	}},
	{remarkPattern(`P(\d{4})`), func(m []string) string {
		return describePrecipitation("1-hour precipitation", m[1])
	}},
	{remarkPattern(`4/(\d{3})`), func(m []string) string {
		return fmt.Sprintf("snow depth: %d inches", atoi(m[1]))
	}},
	{remarkPattern(`933(\d{3})`), func(m []string) string {
		return fmt.Sprintf("water equivalent of snow on ground: %.1f inches", float64(atoi(m[1]))/10)
	}},
	{remarkPattern(`931(\d{3})`), func(m []string) string {
		return fmt.Sprintf("snowfall in the last 6 hours: %.1f inches", float64(atoi(m[1]))/10)
	}},
	{remarkPattern(`98(\d{3})`), func(m []string) string {
		return fmt.Sprintf("sunshine duration on the previous day: %d minutes", atoi(m[1]))
	}},
	{remarkPattern(`I([136])(\d{3})`), func(m []string) string {
		return fmt.Sprintf("%s-hour ice accretion: %.2f inches", m[1], float64(atoi(m[2]))/100)
	}},
	{remarkPattern(`I([136])///`), func(m []string) string {
		return m[1] + "-hour ice accretion not available"
	}},
	{remarkPattern(`8/([\d/])([\d/])([\d/])`), func(m []string) string {
		return fmt.Sprintf("cloud types: low %s, middle %s, high %s",
			describeCloudType(lowCloudTypes, m[1]), describeCloudType(middleCloudTypes, m[2]), describeCloudType(highCloudTypes, m[3]))
	}},
	{remarkPattern(`SNINCR (\d+)/(\d+)`), func(m []string) string {
		return fmt.Sprintf("snow increasing rapidly: %d inches in the past hour, %d inches on ground", atoi(m[1]), atoi(m[2]))
	}},
	{remarkPattern(`(PCPN|WX|T|DP|CLD|PRES|SLP|DENSITY ALT) MISG`), func(m []string) string {
		subject := map[string]string{
			"PCPN":        "precipitation",
			"WX":          "weather",
			"T":           "temperature",
			"DP":          "dew point",
			"CLD":         "cloud",
			"PRES":        "pressure",
			"SLP":         "sea level pressure",
			"DENSITY ALT": "density altitude",
		}[m[1]]
		return subject + " data missing"
	}},
	{remarkPattern(`ICE(?: (INTMT|PAST HR))?`), func(m []string) string {
		switch m[1] {
		case "INTMT":
			return "intermittent icing detected"
		case "PAST HR":
			return "icing detected in the past hour"
		}
		return "icing detected"
	}},
	{remarkPattern(`COR (\d{2})(\d{2})`), func(m []string) string {
		return fmt.Sprintf("correction issued at %s:%s", m[1], m[2])
	}},
	{remarkPattern(`ACFT MSHP`), func(m []string) string {
		return "aircraft mishap"
	}},
	{remarkPattern(`(?:(` + rmkFrequency + `) )?LTN?G((?:IC|CC|CG|CA)*)(?: (` + rmkLocation + `))?`), describeLightning},
	{remarkPattern(`(?:[A-Z]+?(?:[BE](?:\d{4}|\d{2}))+)+`), describeWeatherEvents},
	{remarkPattern(`RE(` + rmkWeather + `)`), func(m []string) string {
		return "recent " + formatWeatherElement(m[1])
	}},
	{remarkPattern(`(?:(` + rmkFrequency + `) )?((?:` + rmkPhenomenon + `)(?: AND (?:` + rmkPhenomenon + `))*)(?: (?:TO )?(` + rmkLocation + `))?(?: (MOVD?) (` + rmkDirections + `|OHD|STNRY))?`), describePhenomenon},
	// Canadian automatic stations send the 3-hour pressure tendency as appp,
	// without the leading 5 of the FMH-1 group.
	{remarkPattern(`([0-8])(\d{3})`), func(m []string) string {
		return fmt.Sprintf("3-hour pressure tendency: %s, %.1f hPa change", pressureTendencies[m[1]], float64(atoi(m[2]))/10)
	}},
	{remarkPattern(`R\d{2}[LCR]?/\S+`), func(m []string) string {
		return "runway visual range information"
	}},
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func signedTenths(sign, value string) float64 {
	tenths := float64(atoi(value)) / 10
	if sign == "1" {
		return -tenths
	}
	return tenths
}

func describeRemarkTime(hour, minute string) string {
	if hour == "" {
		return fmt.Sprintf("at %d minutes past the hour", atoi(minute))
	}
	return fmt.Sprintf("at %s:%s", hour, minute)
}

func describePrecipitation(period, hundredths string) string {
	amount := atoi(hundredths)
	if amount == 0 {
		return period + ": trace"
	}
	return fmt.Sprintf("%s: %.2f inches", period, float64(amount)/100)
}

func describeStatuteMiles(visibility string) string {
	if len(visibility) == 4 && !strings.Contains(visibility, "/") {
		return fmt.Sprintf("%d meters", atoi(visibility))
	}
	if strings.HasPrefix(visibility, "M") {
		return "less than " + visibility[1:] + " statute miles"
	}
	return visibility + " statute miles"
}

func describeCloudType(types []string, code string) string {
	if code == "/" {
		return "not observable"
	}
	return types[atoi(code)]
}

func describeRemarkLocation(location string) string {
	words := strings.Fields(location)
	for i, word := range words {
		if word == "RWY" {
			words[i] = "at runway"
			continue
		}
		if strings.HasPrefix(word, "RWY") {
			words[i] = "at runway " + strings.TrimPrefix(word, "RWY")
			continue
		}
		directions := strings.Split(word, "-")
		for j, direction := range directions {
			if described, ok := remarkLocationWords[direction]; ok {
				directions[j] = described
			}
		}
		words[i] = strings.Join(directions, " through ")
	}
	return strings.Join(words, " ")
}

func describeRemarkWind(m []string) string {
	direction, speed, gust, unit := m[0], m[1], m[2], m[3]
	if speed == "//" {
		return "not available"
	}
	units := map[string]string{"KT": "knots", "MPS": "m/s"}[unit]
	if atoi(speed) == 0 && gust == "" {
		return "calm"
	}
	from := "from " + direction + "°"
	switch direction {
	case "VRB":
		from = "variable"
	case "///":
		from = "direction not available"
	}
	description := fmt.Sprintf("%s at %d %s", from, atoi(speed), units)
	if gust != "" {
		description += fmt.Sprintf(", gusting %d %s", atoi(gust), units)
	}
	return description
}

func describeWindPlace(place string) string {
	switch {
	case place == "ARP/":
		return "aerodrome reference point"
	case place == "MID":
		return "mid-runway"
	case strings.HasPrefix(place, "END R"):
		return "runway " + strings.TrimSuffix(strings.TrimPrefix(place, "END R"), "/") + " end"
	case strings.HasPrefix(place, "R"), strings.HasPrefix(place, "WIND R"):
		return "runway " + strings.TrimSuffix(strings.TrimLeft(strings.TrimPrefix(place, "WIND "), "RWY "), "/")
	case strings.HasPrefix(place, "WIND "):
		return strings.TrimPrefix(place, "WIND ")
	}
	return strings.TrimSuffix(place, " WIND")
}

func describeEstimated(subjects string) string {
	names := map[string]string{
		"WND":   "wind",
		"WIND":  "wind",
		"ALSTG": "altimeter setting",
		"ALTSG": "altimeter setting",
		"SLP":   "sea level pressure",
	}
	var described []string
	for _, subject := range strings.FieldsFunc(subjects, func(r rune) bool { return r == ' ' || r == '/' }) {
		if name, ok := names[subject]; ok {
			described = append(described, name)
		}
	}
	return strings.Join(described, " and ")
}

func describeColorState(m []string) string {
	if m[0] == "BLACK" {
		return "military color state black: airfield not usable for reasons other than weather"
	}
	description := "military color state " + militaryColorStates[m[2]]
	if m[1] != "" {
		description += "; airfield not usable for reasons other than weather"
	}
	return description
}

func describeCloudLayers(m []string) string {
	var layers []string
	for _, layer := range remarkCloudLayerRegex.FindAllStringSubmatch(m[0], -1) {
		layers = append(layers, fmt.Sprintf("%s %s/8", remarkCloudNames[layer[1]], layer[2]))
	}
	return "cloud layer opacity: " + strings.Join(layers, ", ")
}

func describeLightning(m []string) string {
	description := "lightning"
	if m[1] != "" {
		description = remarkFrequencies[m[1]] + " " + description
	}
	if m[2] != "" {
		var types []string
		for i := 0; i+2 <= len(m[2]); i += 2 {
			types = append(types, remarkLightningTypes[m[2][i:i+2]])
		}
		description += " (" + strings.Join(types, ", ") + ")"
	}
	if m[3] != "" {
		description += " " + describeRemarkLocation(m[3])
	}
	return description
}

func describeWeatherEvents(m []string) string {
	var phenomena []string
	for _, event := range remarkEventRegex.FindAllStringSubmatch(m[0], -1) {
		phenomenon := event[1]
		if !remarkEventPhenomenon.MatchString(phenomenon) {
			return ""
		}
		if strings.HasSuffix(phenomenon, "PE") {
			phenomenon = strings.TrimSuffix(phenomenon, "PE") + "PL"
		}
		var times []string
		for _, t := range remarkEventTimeRegex.FindAllStringSubmatch(event[2], -1) {
			action := "began"
			if t[1] == "E" {
				action = "ended"
			}
			hour, minute := "", t[2]
			if len(minute) == 4 {
				hour, minute = minute[:2], minute[2:]
			}
			times = append(times, action+" "+describeRemarkTime(hour, minute))
		}
		phenomena = append(phenomena, formatWeatherElement(phenomenon)+" "+strings.Join(times, ", "))
	}
	return strings.Join(phenomena, "; ")
}

func describePhenomenon(m []string) string {
	var phenomena []string
	for _, phenomenon := range strings.Split(m[2], " AND ") {
		if name, ok := remarkCloudNames[phenomenon]; ok {
			phenomena = append(phenomena, name)
		} else {
			phenomena = append(phenomena, formatWeatherElement(phenomenon))
		}
	}
	description := strings.Join(phenomena, " and ")
	if m[1] != "" {
		description = remarkFrequencies[m[1]] + " " + description
	}
	if m[3] != "" {
		description += " " + describeRemarkLocation(m[3])
	}
	if m[4] != "" {
		verb := "moving"
		if m[4] == "MOVD" {
			verb = "moved"
		}
		description += " " + verb + " " + describeRemarkLocation(m[5])
	}
	return description
}

// decodeRemarkGroup decodes the remark group at the start of parts and
// returns its raw text and the number of tokens it consumed.
func decodeRemarkGroup(parts []string) (raw string, consumed int, description string) {
	text := strings.Join(parts, " ")
	for _, rule := range remarkRules {
		match := rule.pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		match[0] = strings.TrimSuffix(match[0], " ")
		if described := rule.describe(match); described != "" {
			return match[0], len(strings.Fields(match[0])), described
		}
	}
	return "", 0, ""
}

// processRemarks processes the remarks section of a METAR
func processRemarks(remarkParts []string) []Remark {
	remarks := []Remark{}
	remarkCodes := newRemarkCodes()

	var plainLanguage []string
	flushPlainLanguage := func() {
		if len(plainLanguage) > 0 {
			remarks = append(remarks, Remark{
				Raw:         strings.Join(plainLanguage, " "),
				Description: "plain-language remark",
			})
			plainLanguage = nil
		}
	}

	for i := 0; i < len(remarkParts); {
		part := remarkParts[i]

		if raw, consumed, description := decodeRemarkGroup(remarkParts[i:]); consumed > 0 {
			flushPlainLanguage()
			remarks = append(remarks, Remark{Raw: raw, Description: description})
			i += consumed
			continue
		}

		if description, ok := remarkCodes[part]; ok {
			flushPlainLanguage()
			remarks = append(remarks, Remark{Raw: part, Description: description})
			i++
			continue
		}

		if remarkPlainLanguageRegex.MatchString(part) {
			plainLanguage = append(plainLanguage, part)
			i++
			continue
		}

		flushPlainLanguage()
		remarks = append(remarks, Remark{
			Raw:         part,
			Description: "unknown remark code",
		})
		i++
	}
	flushPlainLanguage()

	return remarks
}

// processMETAR prints the raw METAR and its decoding with the given site information
func processMETAR(rawMetar string, siteInfo SiteInfo, noRaw bool, noDecode bool) {
	if !noRaw {
		functionColor.Println("----- Raw METAR -----")
		fmt.Println(rawMetar)
		if !noDecode {
			fmt.Println()
		}
	}

	if !noDecode {
		metar := DecodeMETAR(rawMetar, time.Now().UTC())
		metar.SiteInfo = siteInfo
		functionColor.Println("--- Decoded METAR ---")
		fmt.Print(FormatMETAR(metar))
	}
}

// processTAF prints the raw TAF and its decoding with the given site information
func processTAF(rawTAF string, siteInfo SiteInfo, noRaw bool, noDecode bool) {
	if !noRaw {
		functionColor.Println("------ Raw TAF ------")
		fmt.Println(rawTAF)
		if !noDecode {
			fmt.Println()
		}
	}

	if !noDecode {
		taf := DecodeTAF(rawTAF, time.Now().UTC())
		taf.SiteInfo = siteInfo
		functionColor.Println("---- Decoded TAF ----")
		fmt.Print(FormatTAF(taf))
	}
}
