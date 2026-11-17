package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"k8s.io/utils/ptr"
)

// conditions collects every group the METAR body and forecast period grammars decode. METAR and
// Forecast copy out the fields they carry.
type conditions struct {
	reference        time.Time
	wind             Wind
	windVariation    string
	windShear        []WindShear
	visibility       string
	minVisibility    string
	cavok            bool
	runwayConditions []RunwayCondition
	runwayStates     []RunwayState
	weather          []string
	nsw              bool
	clouds           []Cloud
	nsc              bool
	vertVis          int
	vertVisMissing   bool
	temperature      *int
	dewPoint         *int
	temperatures     []TemperatureForecast
	pressure         float64
	pressureUnit     string
	stationPressure  float64
	seaStates        []SeaState
	icing            []HazardLayer
	turbulence       []HazardLayer
	specialCodes     []string
	unhandled        []string
}

// group is one report group: an anchored pattern matched against span tokens joined by spaces.
// apply returns false to reject a match the pattern alone cannot rule out.
type group struct {
	span    int
	pattern *regexp.Regexp
	apply   func(c *conditions, match []string) bool
}

func whole(set func(c *conditions, raw string)) func(*conditions, []string) bool {
	return func(c *conditions, match []string) bool {
		set(c, match[0])
		return true
	}
}

func appendSpecialCode(c *conditions, code string) {
	c.specialCodes = append(c.specialCodes, code)
}

var (
	modifierRegex        = regexp.MustCompile(`^(AUTO|COR|CCA|NIL|RTD)$`)
	missingGroupRegex    = regexp.MustCompile(`^M$`)
	visMissingRegex      = regexp.MustCompile(`^////(SM)?$`)
	splitVisibilityRegex = regexp.MustCompile(`^\d \d/\d{1,2}SM$`)
	weatherMissingRegex  = regexp.MustCompile(`^(RE)?//$`)
	nswRegex             = regexp.MustCompile(`^NSW$`)
	nscRegex             = regexp.MustCompile(`^NSC$`)
	ncdRegex             = regexp.MustCompile(`^NCD$`)
	vvMissingRegex       = regexp.MustCompile(`^VV///$`)
	tempMissingRegex     = regexp.MustCompile(`^/////$`)
	pressureHPaRegex     = regexp.MustCompile(`^Q(\d+)$`)
	nosigRegex           = regexp.MustCompile(`^NOSIG$`)
	windNoteRegex        = regexp.MustCompile(`^WND( \S+ AFT \S+| \S+)?$`)
)

var (
	modifierGroup     = group{1, modifierRegex, whole(appendSpecialCode)}
	missingGroupGroup = group{1, missingGroupRegex, whole(func(c *conditions, raw string) {
		if !slices.Contains(c.specialCodes, raw) {
			appendSpecialCode(c, raw)
		}
	})}

	windGroups = []group{
		{1, windRegex, whole(func(c *conditions, raw string) { c.wind = parseWind(raw) })},
		{1, windRegexMPS, whole(func(c *conditions, raw string) { c.wind = parseWind(raw) })},
		{1, windVarRegex, whole(func(c *conditions, raw string) { c.windVariation = raw })},
	}

	setVisibility = whole(func(c *conditions, raw string) { c.visibility = raw })
	cavokGroup    = group{1, cavokRegex, whole(func(c *conditions, raw string) {
		c.cavok = true
		c.visibility = raw
		appendSpecialCode(c, raw)
	})}
	visibilityGroups = []group{
		{2, splitVisibilityRegex, setVisibility},
		{1, visRegexDir, whole(func(c *conditions, raw string) {
			if c.visibility != "" {
				c.minVisibility = raw
			} else {
				c.visibility = raw
			}
		})},
		{1, visRegexSM, setVisibility},
		{1, visRegexNum, setVisibility},
		{1, ndvRegex, setVisibility},
		cavokGroup,
	}
	visibilityMissingGroup = group{1, visMissingRegex, whole(func(c *conditions, raw string) {
		if c.visibility == "" {
			c.visibility = "////"
		}
	})}

	runwayGroups = []group{
		{1, runwayStateRegex, appendRunwayState},
		{1, runwayClearedRegex, appendRunwayState},
		{1, runwaySnowClosedRegex, appendRunwayState},
		{1, runwayCondRegex, appendRunwayCondition},
		{1, rvrMissingRegex, appendRunwayCondition},
	}

	weatherGroup = group{1, weatherGroupRegex, func(c *conditions, match []string) bool {
		if !isWeatherCode(match[0]) {
			return false
		}
		c.weather = append(c.weather, match[0])
		return true
	}}
	weatherMissingGroup = group{1, weatherMissingRegex, whole(func(c *conditions, raw string) { c.weather = append(c.weather, raw) })}
	nswGroup            = group{1, nswRegex, whole(func(c *conditions, raw string) { c.nsw = true })}

	vertVisGroup = group{1, vvRegex, func(c *conditions, match []string) bool {
		c.vertVis, _ = strconv.Atoi(match[1])
		return true
	}}
	vertVisMissingGroup = group{1, vvMissingRegex, whole(func(c *conditions, raw string) { c.vertVisMissing = true })}
	nscGroup            = group{1, nscRegex, whole(func(c *conditions, raw string) {
		c.nsc = true
		appendSpecialCode(c, raw)
	})}
	ncdGroup   = group{1, ncdRegex, whole(appendSpecialCode)}
	cloudGroup = group{1, cloudRegex, whole(func(c *conditions, raw string) { c.clouds = append(c.clouds, parseCloud(raw)) })}

	temperatureGroups = []group{
		{1, tempRegex, func(c *conditions, match []string) bool {
			c.temperature = ptr.To(signedCelsius(match[1], match[2]))
			c.dewPoint = ptr.To(signedCelsius(match[3], match[4]))
			return true
		}},
		{1, tempOnlyRegex, func(c *conditions, match []string) bool {
			c.temperature = ptr.To(signedCelsius(match[1], match[2]))
			return true
		}},
		{1, tempMissingRegex, whole(func(c *conditions, raw string) {})},
	}
	forecastTemperatureGroup = group{1, tafTemperatureRegex, func(c *conditions, match []string) bool {
		temperature, ok := parseTemperatureForecast(match, c.reference)
		if ok {
			c.temperatures = append(c.temperatures, temperature)
		}
		return ok
	}}

	// Only the first pressure group counts; some stations report QNH in both hPa and inHg
	pressureGroups = []group{
		{1, pressureMissingRegex, whole(func(c *conditions, raw string) {
			if c.pressureUnit == "" {
				c.pressureUnit = "hPa"
				if raw[0] == 'A' {
					c.pressureUnit = "inHg"
				}
			}
		})},
		{1, pressureHPaRegex, func(c *conditions, match []string) bool {
			if c.pressureUnit == "" {
				hectopascals, _ := strconv.Atoi(match[1])
				c.pressure, c.pressureUnit = float64(hectopascals), "hPa"
			}
			return true
		}},
		{1, pressureRegex, func(c *conditions, match []string) bool {
			if c.pressureUnit == "" {
				hundredths, _ := strconv.Atoi(match[1])
				c.pressure, c.pressureUnit = float64(hundredths)/100.0, "inHg"
			}
			return true
		}},
	}
	stationPressureGroups = []group{
		{2, qfeRegex, appendStationPressure},
		{1, qfeRegex, appendStationPressure},
	}
	qnhInchesGroup = group{1, tafQNHInchesRegex, func(c *conditions, match []string) bool {
		hundredths, _ := strconv.Atoi(match[1])
		c.pressure, c.pressureUnit = float64(hundredths)/100.0, "inHg"
		return true
	}}

	windShearGroups = []group{
		{3, windShearRwyRegex, appendWindShear},
		{2, windShearRwyRegex, appendWindShear},
		{1, windShearAltRegex, appendWindShear},
	}

	seaStateGroup = group{1, seaStateRegex, whole(func(c *conditions, raw string) { c.seaStates = append(c.seaStates, parseSeaState(raw)) })}

	hazardGroups = []group{
		{1, tafIcingRegex, func(c *conditions, match []string) bool {
			c.icing = append(c.icing, parseHazardLayer(match, match[0]))
			return true
		}},
		{1, tafTurbulenceRegex, func(c *conditions, match []string) bool {
			c.turbulence = append(c.turbulence, parseHazardLayer(match, match[0]))
			return true
		}},
	}

	militaryColorGroup = group{1, militaryColorRegex, whole(appendSpecialCode)}
	nosigGroup         = group{1, nosigRegex, whole(appendSpecialCode)}

	// US military TAFs add notes such as "WND 02006KT AFT 0801" that must not replace the period wind
	windNoteGroups = []group{
		{4, windNoteRegex, appendWindNote},
		{2, windNoteRegex, appendWindNote},
		{1, windNoteRegex, appendWindNote},
	}
)

func appendStationPressure(c *conditions, match []string) bool {
	c.stationPressure, _ = strconv.ParseFloat(strings.Replace(match[1], "/", ".", 1), 64)
	return true
}

func appendRunwayState(c *conditions, match []string) bool {
	c.runwayStates = append(c.runwayStates, parseRunwayState(match[0]))
	return true
}

func appendRunwayCondition(c *conditions, match []string) bool {
	c.runwayConditions = append(c.runwayConditions, parseRunwayCondition(match[0]))
	return true
}

func appendWindShear(c *conditions, match []string) bool {
	c.windShear = append(c.windShear, parseWindShear(match[0]))
	return true
}

func appendWindNote(c *conditions, match []string) bool {
	c.unhandled = append(c.unhandled, strings.Fields(match[0])...)
	return true
}

func signedCelsius(sign, digits string) int {
	celsius, _ := strconv.Atoi(digits)
	if sign == "M" {
		return -celsius
	}
	return celsius
}

// metarBodyGrammar lists the METAR body groups in WMO No. 306 FM 15 order. Runway state groups
// precede RVR because R24/290050 also matches the RVR pattern.
var metarBodyGrammar = slices.Concat(
	[]group{modifierGroup, missingGroupGroup},
	windGroups,
	[]group{visibilityMissingGroup},
	visibilityGroups,
	runwayGroups,
	[]group{weatherGroup, weatherMissingGroup},
	[]group{vertVisGroup, vertVisMissingGroup, nscGroup, ncdGroup, cloudGroup},
	temperatureGroups,
	pressureGroups,
	stationPressureGroups,
	windShearGroups,
	[]group{seaStateGroup, nosigGroup, militaryColorGroup},
)

// periodGrammar lists the groups of a TAF period (WMO No. 306 FM 51) and of a METAR trend group,
// plus the US military TAF temperature, QNH, icing, and turbulence groups
var periodGrammar = slices.Concat(
	windGroups,
	visibilityGroups,
	[]group{weatherGroup, nswGroup},
	[]group{vertVisGroup, nscGroup, cloudGroup},
	[]group{forecastTemperatureGroup, qnhInchesGroup},
	windShearGroups,
	hazardGroups,
	windNoteGroups,
)

// decodeGroups decodes tokens with the first group in grammar that matches at each position.
// Tokens no group matches go to unhandled.
func decodeGroups(tokens []string, grammar []group, reference time.Time) conditions {
	c := conditions{reference: reference}
	for i := 0; i < len(tokens); {
		consumed := c.decodeGroup(tokens[i:], grammar)
		if consumed == 0 {
			c.unhandled = append(c.unhandled, tokens[i])
			consumed = 1
		}
		i += consumed
	}
	return c
}

func (c *conditions) decodeGroup(tokens []string, grammar []group) int {
	for _, g := range grammar {
		if g.span > len(tokens) {
			continue
		}
		match := g.pattern.FindStringSubmatch(strings.Join(tokens[:g.span], " "))
		if match != nil && g.apply(c, match) {
			return g.span
		}
	}
	return 0
}

func (c conditions) applyToForecast(f *Forecast) {
	f.Wind = c.wind
	f.WindVariation = c.windVariation
	f.WindShear = c.windShear
	f.Visibility = c.visibility
	f.MinVisibility = c.minVisibility
	f.CAVOK = c.cavok
	f.Weather = c.weather
	f.NSW = c.nsw
	f.Clouds = c.clouds
	f.NSC = c.nsc
	f.VertVis = c.vertVis
	f.Pressure = c.pressure
	f.PressureUnit = c.pressureUnit
	f.Icing = c.icing
	f.Turbulence = c.turbulence
	f.Unhandled = c.unhandled
}

func (c conditions) applyToMETAR(m *METAR) {
	m.Wind = c.wind
	m.WindVariation = c.windVariation
	m.WindShear = c.windShear
	m.Visibility = c.visibility
	m.MinVisibility = c.minVisibility
	m.RunwayConditions = c.runwayConditions
	m.RunwayStates = c.runwayStates
	m.Weather = c.weather
	m.Clouds = c.clouds
	m.VertVis = c.vertVis
	m.VertVisMissing = c.vertVisMissing
	m.Temperature = c.temperature
	m.DewPoint = c.dewPoint
	m.Pressure = c.pressure
	m.PressureUnit = c.pressureUnit
	m.StationPressure = c.stationPressure
	m.SeaStates = c.seaStates
	m.SpecialCodes = append(m.SpecialCodes, c.specialCodes...)
	m.Unhandled = c.unhandled
}

// changeGroup is a forecast period that starts with a change group header
type changeGroup struct {
	header  Forecast
	tokens  []string
	content []string
}

// splitChangeGroups returns the tokens before the first change group header and each change group
func splitChangeGroups(tokens []string, reference time.Time) ([]string, []changeGroup) {
	type headerPosition struct {
		header     Forecast
		start, end int
	}
	var headers []headerPosition
	for i := 0; i < len(tokens); {
		header, consumed, ok := parseChangeGroup(tokens, i, reference)
		if !ok {
			i++
			continue
		}
		headers = append(headers, headerPosition{header, i, i + consumed})
		i += consumed
	}

	if len(headers) == 0 {
		return tokens, nil
	}
	groups := make([]changeGroup, len(headers))
	for k, position := range headers {
		end := len(tokens)
		if k+1 < len(headers) {
			end = headers[k+1].start
		}
		groups[k] = changeGroup{header: position.header, tokens: tokens[position.start:end], content: tokens[position.end:end]}
	}
	return tokens[:headers[0].start], groups
}

// decodePeriod decodes a change group's content with periodGrammar into its header Forecast
func (g changeGroup) decodePeriod(reference time.Time) (Forecast, conditions) {
	forecast := g.header
	forecast.Raw = strings.Join(g.tokens, " ")
	decoded := decodeGroups(g.content, periodGrammar, reference)
	decoded.applyToForecast(&forecast)
	return forecast, decoded
}
