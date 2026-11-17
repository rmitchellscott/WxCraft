package main

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"time"

	"k8s.io/utils/ptr"
)

// Flight categories from worst to best, with the FAA thresholds from the Aeronautical Information
// Manual's Categorical Outlooks: LIFR is a ceiling below 500 feet or visibility below 1 mile, IFR a
// ceiling of 500 to below 1,000 feet or visibility of 1 to below 3 miles, MVFR a ceiling of 1,000
// to 3,000 feet or visibility of 3 to 5 miles, and VFR a ceiling above 3,000 feet (or none) and
// visibility above 5 miles
const (
	lifr = iota
	ifr
	mvfr
	vfr
)

var flightCategoryNames = []string{lifr: "LIFR", ifr: "IFR", mvfr: "MVFR", vfr: "VFR"}

// Meter visibilities are reported in ICAO steps whose conventional statute-mile equivalents are
// 1600 m for 1 mile, 4800 m for 3 and 8000 m for 5. The exact 1609.344 would put 1600 m below 1 mile.
const metersPerReportedMile = 1600.0

// stations.json gives 9999 meters for stations, mostly buoys, whose elevation is unknown
const unknownStationElevation = 9999

var (
	statuteMilesRegex       = regexp.MustCompile(`^([MP])?(?:(\d+) )?(\d+)(?:/(\d+))?SM$`)
	meterVisibilityRegex    = regexp.MustCompile(`^(\d{4})(?:NDV|[NESW]{1,2})?$`)
	tenthsTemperatureRemark = regexp.MustCompile(`^T([01])(\d{3})(?:([01])(\d{3}))?`)
)

// skyCondition holds the groups that set the ceiling and flight category of a report or period
type skyCondition struct {
	visibility     string
	clouds         []Cloud
	vertVis        *int
	vertVisMissing bool
	noCloud        bool // CAVOK, NSC, or NCD
}

func metarSky(m METAR) skyCondition {
	return skyCondition{
		visibility:     m.Visibility,
		clouds:         m.Clouds,
		vertVis:        m.VertVis,
		vertVisMissing: m.VertVisMissing,
		noCloud: slices.ContainsFunc(m.SpecialCodes, func(code string) bool {
			return code == "CAVOK" || code == "NSC" || code == "NCD"
		}),
	}
}

func forecastSky(f Forecast) skyCondition {
	return skyCondition{
		visibility: f.Visibility,
		clouds:     f.Clouds,
		vertVis:    f.VertVis,
		noCloud:    f.CAVOK || f.NSC,
	}
}

func (s skyCondition) skyReported() bool {
	return s.noCloud || len(s.clouds) > 0 || s.vertVis != nil || s.vertVisMissing
}

// overlay applies a change period's visibility and sky to the conditions it changes; a change
// period that omits one keeps the one it changes
func (s skyCondition) overlay(change skyCondition) skyCondition {
	if change.visibility != "" {
		s.visibility = change.visibility
	}
	if change.skyReported() {
		s.clouds, s.vertVis, s.vertVisMissing, s.noCloud = change.clouds, change.vertVis, change.vertVisMissing, change.noCloud
	}
	return s
}

// ceiling is the height of the lowest broken or overcast layer or of the vertical visibility
type ceiling struct {
	feet int
	none bool
}

// ceiling returns false when the sky is not reported or a layer below the ceiling has no coverage
// or height, since that layer could be the ceiling
func (s skyCondition) ceiling() (ceiling, bool) {
	layerFeet := -1
	for _, cloud := range s.clouds {
		if cloud.Coverage == "///" {
			return ceiling{}, false
		}
		if cloud.Coverage == "BKN" || cloud.Coverage == "OVC" {
			if cloud.HeightMissing {
				return ceiling{}, false
			}
			layerFeet = cloud.Height
			break
		}
	}

	switch {
	case s.vertVis != nil && (layerFeet < 0 || *s.vertVis*100 < layerFeet):
		return ceiling{feet: *s.vertVis * 100}, true
	case layerFeet >= 0:
		return ceiling{feet: layerFeet}, true
	case s.vertVisMissing || !s.skyReported():
		return ceiling{}, false
	}
	return ceiling{none: true}, true
}

// statuteMiles is a visibility in statute miles; lessThan and moreThan mark an M or P prefix
type statuteMiles struct {
	miles    float64
	lessThan bool
	moreThan bool
}

func parseStatuteMiles(visibility string) (statuteMiles, bool) {
	if visibility == "CAVOK" {
		return statuteMiles{miles: 10000 / metersPerReportedMile, moreThan: true}, true
	}

	if matches := meterVisibilityRegex.FindStringSubmatch(visibility); matches != nil {
		meters, _ := strconv.Atoi(matches[1])
		switch meters {
		case 0:
			return statuteMiles{miles: 50 / metersPerReportedMile, lessThan: true}, true
		case 9999:
			return statuteMiles{miles: 10000 / metersPerReportedMile, moreThan: true}, true
		}
		return statuteMiles{miles: float64(meters) / metersPerReportedMile}, true
	}

	matches := statuteMilesRegex.FindStringSubmatch(visibility)
	if matches == nil {
		return statuteMiles{}, false
	}
	whole, _ := strconv.Atoi(matches[2])
	numerator, _ := strconv.Atoi(matches[3])
	miles := float64(whole + numerator)
	if matches[4] != "" {
		denominator, _ := strconv.Atoi(matches[4])
		if denominator == 0 {
			return statuteMiles{}, false
		}
		miles = float64(whole) + float64(numerator)/float64(denominator)
	}
	return statuteMiles{miles: miles, lessThan: matches[1] == "M", moreThan: matches[1] == "P"}, true
}

func visibilityCategory(v statuteMiles) int {
	switch {
	case v.miles < 1 || v.lessThan && v.miles <= 1:
		return lifr
	case v.miles < 3 || v.lessThan && v.miles <= 3:
		return ifr
	case v.miles < 5 || v.miles == 5 && !v.moreThan:
		return mvfr
	}
	return vfr
}

func ceilingCategory(c ceiling) int {
	switch {
	case c.none:
		return vfr
	case c.feet < 500:
		return lifr
	case c.feet < 1000:
		return ifr
	case c.feet <= 3000:
		return mvfr
	}
	return vfr
}

// flightCategory is the worse of the ceiling and visibility categories. With one of them unknown
// the category is known only when the other is already LIFR.
func (s skyCondition) flightCategory() string {
	worst, known := vfr, 0
	if c, ok := s.ceiling(); ok {
		worst, known = min(worst, ceilingCategory(c)), known+1
	}
	if v, ok := parseStatuteMiles(s.visibility); ok {
		worst, known = min(worst, visibilityCategory(v)), known+1
	}
	if known == 2 || known == 1 && worst == lifr {
		return flightCategoryNames[worst]
	}
	return ""
}

// periodSkies returns the visibility and sky in force during each TAF period. A BECMG change
// lasts until the next BASE or FM period; TEMPO, INTER, and PROB changes apply only to their own period.
func periodSkies(forecasts []Forecast) []skyCondition {
	skies := make([]skyCondition, len(forecasts))
	var prevailing skyCondition
	for i, forecast := range forecasts {
		sky := forecastSky(forecast)
		switch forecast.Type {
		case "BASE", "FM":
			prevailing = sky
			skies[i] = sky
		case "BECMG":
			prevailing = prevailing.overlay(sky)
			skies[i] = prevailing
		default:
			skies[i] = prevailing.overlay(sky)
		}
	}
	return skies
}

// markPeriodsInEffect flags the BASE or FM period that holds at reference and the change periods
// active then. A BECMG change stays in effect after its transition until the next BASE or FM period.
func markPeriodsInEffect(forecasts []Forecast, reference time.Time) {
	var prevailingEnd time.Time
	for i := range forecasts {
		forecast := &forecasts[i]
		switch forecast.Type {
		case "BASE", "FM":
			prevailingEnd = forecast.To
			forecast.InEffect = activeAt(forecast.From, forecast.To, reference)
		case "BECMG":
			forecast.InEffect = activeAt(forecast.From, prevailingEnd, reference)
		default:
			forecast.InEffect = activeAt(forecast.From, forecast.To, reference)
		}
	}
}

func activeAt(from, to, reference time.Time) bool {
	return !from.IsZero() && !reference.Before(from) && (to.IsZero() || reference.Before(to))
}

// metarDerived holds values computed from a decoded METAR and its station information
type metarDerived struct {
	ceiling          ceiling
	ceilingKnown     bool
	flightCategory   string
	temperature      *float64
	dewPoint         *float64
	tenths           bool // temperature and dew point come from the T remark group
	relativeHumidity *float64
	densityAltitude  *int
}

func deriveMETAR(m METAR) metarDerived {
	sky := metarSky(m)
	var d metarDerived
	d.ceiling, d.ceilingKnown = sky.ceiling()
	d.flightCategory = sky.flightCategory()

	if m.Temperature != nil {
		d.temperature = ptr.To(float64(*m.Temperature))
	}
	if m.DewPoint != nil {
		d.dewPoint = ptr.To(float64(*m.DewPoint))
	}
	for _, remark := range m.Remarks {
		if matches := tenthsTemperatureRemark.FindStringSubmatch(remark.Raw); matches != nil {
			d.tenths = true
			d.temperature = ptr.To(signedTenths(matches[1], matches[2]))
			if matches[3] != "" {
				d.dewPoint = ptr.To(signedTenths(matches[3], matches[4]))
			}
			break
		}
	}

	if d.temperature != nil && d.dewPoint != nil {
		d.relativeHumidity = ptr.To(relativeHumidity(*d.temperature, *d.dewPoint))
	}

	elevation := m.SiteInfo.Elevation
	if d.temperature != nil && elevation != nil && *elevation != unknownStationElevation && m.Pressure > 0 {
		altimeter := m.Pressure
		if m.PressureUnit == "hPa" {
			altimeter = MillibarsToInHg(m.Pressure)
		}
		feet := int(math.Round(densityAltitude(float64(*elevation)/0.3048, altimeter, *d.temperature)))
		d.densityAltitude = &feet
	}
	return d
}

// relativeHumidity uses the Magnus formula with the Alduchov and Eskridge (1996) constants
// a = 17.625 and b = 243.04 °C: RH = 100 * exp(a*Td/(b+Td) - a*T/(b+T))
func relativeHumidity(celsius, dewPointCelsius float64) float64 {
	const a, b = 17.625, 243.04
	rh := 100 * math.Exp(a*dewPointCelsius/(b+dewPointCelsius)-a*celsius/(b+celsius))
	return math.Min(rh, 100)
}

// densityAltitude uses the FAA Pilot's Handbook of Aeronautical Knowledge (FAA-H-8083-25)
// approximations: pressure altitude = elevation + (29.92 - altimeter) * 1000, standard
// temperature = 15 - 2 °C per 1,000 feet, and density altitude = pressure altitude +
// 120 * (temperature - standard temperature)
func densityAltitude(elevationFeet, altimeterInHg, celsius float64) float64 {
	pressureAltitude := elevationFeet + (29.92-altimeterInHg)*1000
	standardCelsius := 15 - 2*pressureAltitude/1000
	return pressureAltitude + 120*(celsius-standardCelsius)
}
