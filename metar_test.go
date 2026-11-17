package main

import (
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

// formatWithoutTime renders a METAR with the observation time cleared, since the
// formatter prints the age relative to time.Now
func formatWithoutTime(m METAR) string {
	color.NoColor = true
	m.Time = time.Time{}
	return FormatMETAR(m)
}

var metarTestReference = time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)

func TestMETARReportTypePrefix(t *testing.T) {
	tests := []struct {
		raw          string
		reportType   string
		station      string
		time         string
		specialCodes []string
	}{
		{"METAR KJFK 121651Z 18010KT 10SM CLR 20/10 A3001", "METAR", "KJFK", "121651Z", nil},
		{"SPECI EGLL 121620Z 24015KT 9999 FEW030 15/08 Q1012", "SPECI", "EGLL", "121620Z", nil},
		{"METAR COR LFPG 121200Z 27008KT CAVOK 18/09 Q1020", "METAR", "LFPG", "121200Z", []string{"COR", "CAVOK"}},
		{"KJFK 121651Z 18010KT 10SM CLR 20/10 A3001", "", "KJFK", "121651Z", nil},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			m := DecodeMETAR(tt.raw, metarTestReference)
			assert.Equal(t, tt.reportType, m.ReportType)
			assert.Equal(t, tt.station, m.Station)
			assert.Equal(t, tt.station, m.SiteInfo.Name)
			assert.Equal(t, tt.time, m.Time.Format("021504Z"))
			assert.Equal(t, tt.specialCodes, m.SpecialCodes)
			assert.Empty(t, m.Unhandled)
		})
	}
}

func TestMETARIsWeatherCode(t *testing.T) {
	weather := []string{
		"-RA", "+TSRA", "VCSH", "VCTS", "TS", "SH", "RERA", "RETSRA", "FZFG", "-FZDZ",
		"BLSN", "DRSN", "MIFG", "BCFG", "PRFG", "+FC", "FC", "-SHRASN", "BR", "HZ",
		"SA", "VCFG", "+SHGSRA", "UP", "-VCTSRA", "SQ", "PO", "VA", "DS", "+SS",
	}
	for _, code := range weather {
		assert.True(t, isWeatherCode(code), code)
	}

	notWeather := []string{
		"BLU", "BLU+", "BLACKBLU", "FZRANO", "0VC100", "SANTGO", "SANTA", "BCGF", "TSVCSH",
		"RERANOSIG", "UNMONITORED", "RE", "VC", "-", "+", "", "WS", "MI", "BL", "FEW020",
		"CLR", "SKC", "OVC010CB", "RMK", "NOSIG", "//", "RE//", "8000-RA",
	}
	for _, code := range notWeather {
		assert.False(t, isWeatherCode(code), code)
	}
}

func TestMETARRunwayStateAndRVR(t *testing.T) {
	m := DecodeMETAR("UWKE 080747Z 33005MPS 9999 OVC008 M02/M02 Q1010 "+
		"R24/290050 R88/CLRD// R06/CLRD62 R07/32//50 R02/////// R/SNOCLO "+
		"R24/P2000N R04/1200V1800FT/U R10///// NOSIG", metarTestReference)

	assert.Equal(t, []RunwayState{
		{Runway: "24", Deposit: "2", Extent: "9", Depth: "00", Friction: "50", Raw: "R24/290050"},
		{Runway: "88", Cleared: true, Friction: "//", Raw: "R88/CLRD//"},
		{Runway: "06", Cleared: true, Friction: "62", Raw: "R06/CLRD62"},
		{Runway: "07", Deposit: "3", Extent: "2", Depth: "//", Friction: "50", Raw: "R07/32//50"},
		{Runway: "02", Deposit: "/", Extent: "/", Depth: "//", Friction: "//", Raw: "R02///////"},
		{SnowClosed: true, Raw: "R/SNOCLO"},
	}, m.RunwayStates)

	assert.Equal(t, []RunwayCondition{
		{Runway: "24", Visibility: 2000, Prefix: "P", Trend: "N", Raw: "R24/P2000N"},
		{Runway: "04", Visibility: 1200, VisMin: 1200, VisMax: 1800, Unit: "FT", Trend: "U", Raw: "R04/1200V1800FT/U"},
		{Runway: "10", Missing: true, Raw: "R10/////"},
	}, m.RunwayConditions)
	assert.Empty(t, m.Unhandled)

	expected := `Station: UWKE
Wind: From 330° at 5 meters per second
Visibility: Unlimited visibility (greater than 10 kilometers)
Clouds: Overcast at 800 feet
Ceiling: 800 feet
Flight Category: IFR
Temperature: -2°C | 28°F
Dew Point: -2°C | 28°F
Relative Humidity: 100%
Pressure: 1010.0 hPa | 29.83 inHg

Runway Visual Range:
  Runway 24: More than 2000 meters (no change)
  Runway 04: Visibility between 1200 and 1800 feet (increasing)
  Runway 10: Not reported

Runway State:
  Runway 24: Wet or water patches, 51-100% covered, depth less than 1 mm, friction coefficient 0.50
  All runways: Contamination cleared
  Runway 06: Contamination cleared, friction coefficient 0.62
  Runway 07: Rime or frost, 11-25% covered, depth not significant or not measurable, friction coefficient 0.50
  Runway 02: Not reported
  Aerodrome: Closed due to snow

Special Conditions:
  • No significant changes expected
`
	assert.Equal(t, expected, formatWithoutTime(m))
}

func TestMETARRunwayStateCodes(t *testing.T) {
	tests := []struct {
		state    RunwayState
		expected string
	}{
		{RunwayState{Runway: "99", Deposit: "7", Extent: "5", Depth: "93", Friction: "91"},
			"Runway (last report repeated): Ice, 26-50% covered, depth 15 cm, braking action poor"},
		{RunwayState{Runway: "27L", Deposit: "/", Extent: "1", Depth: "98", Friction: "99"},
			"Runway 27L: Deposit not reported, 10% or less covered, depth 40 cm or more, braking action unreliable"},
		{RunwayState{Runway: "09", Deposit: "6", Extent: "/", Depth: "99", Friction: "//"},
			"Runway 09: Slush, extent not reported, runway not operational, braking action not reported"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatRunwayState(tt.state))
	}
}

func TestMETARWind(t *testing.T) {
	tests := []struct {
		raw      string
		wind     Wind
		expected string
	}{
		{"00000KT", Wind{Direction: "000", Speed: ptr.To(0), Unit: "KT"}, "Calm"},
		{"VRB00KT", Wind{Direction: "VRB", Speed: ptr.To(0), Unit: "KT"}, "Calm"},
		{"/////KT", Wind{Direction: "///", Unit: "KT"}, "Not reported"},
		{"///05KT", Wind{Direction: "///", Speed: ptr.To(5), Unit: "KT"}, "Direction not reported at 5 knots"},
		{"27015G25KT", Wind{Direction: "270", Speed: ptr.To(15), Gust: 25, Unit: "KT"}, "From 270° at 15 knots, gusting to 25 knots"},
		{"VRB03MPS", Wind{Direction: "VRB", Speed: ptr.To(3), Unit: "MPS"}, "Variable at 3 meters per second"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			m := DecodeMETAR("KXYZ 121651Z "+tt.raw+" 10SM CLR 20/10 A3001", metarTestReference)
			assert.Equal(t, tt.wind, m.Wind)
			assert.Equal(t, tt.expected, formatWind(m.Wind))
			assert.Empty(t, m.Unhandled)
		})
	}

	assert.Equal(t, "", formatWind(Wind{}))
}

func TestMETARWindShearSpeed(t *testing.T) {
	m := DecodeMETAR("KXYZ 121651Z 18010KT 10SM CLR 20/10 A3001 WS020/27045G55KT", metarTestReference)
	assert.Contains(t, formatWithoutTime(m), "\nWind Shear:\n  At 2,000 feet: From 270° at 45 knots, gusting to 55 knots\n")

	color.NoColor = true
	taf := TAF{
		WeatherData: WeatherData{Station: "KXYZ"},
		Forecasts: []Forecast{{
			Type:      "BASE",
			WindShear: []WindShear{parseWindShear("WS015/30040KT")},
		}},
	}
	assert.Contains(t, FormatTAF(taf), "   Wind Shear: At 1,500 feet: From 300° at 40 knots\n")
}

func TestMETARDirectionalMinimumVisibility(t *testing.T) {
	m := DecodeMETAR("LIRF 121650Z 18005KT 4000 1500NE BR SCT010 12/11 Q1018", metarTestReference)
	assert.Equal(t, "4000", m.Visibility)
	assert.Equal(t, "1500NE", m.MinVisibility)
	assert.Equal(t, []string{"BR"}, m.Weather)
	assert.Empty(t, m.Unhandled)
	assert.Contains(t, formatWithoutTime(m),
		"Visibility: 4,000 meters\nMinimum Visibility: 1,500 meters in the NE direction\n")
}

func TestMETARMissingValues(t *testing.T) {
	m := DecodeMETAR("ENUG 080750Z AUTO /////KT ///V/// //// // BKN008/// ///015 ///CB "+
		"///////// BKN/// FEW015/// ///// Q//// W///S4 W15/S2 W12/H7 WM01/// RE//", metarTestReference)

	assert.Equal(t, Wind{Direction: "///", Unit: "KT"}, m.Wind)
	assert.Equal(t, "///V///", m.WindVariation)
	assert.Equal(t, "////", m.Visibility)
	assert.Equal(t, []string{"//", "RE//"}, m.Weather)
	assert.Equal(t, []Cloud{
		{Coverage: "BKN", Height: 800, Type: "///"},
		{Coverage: "///", Height: 1500},
		{Coverage: "///", HeightMissing: true, Type: "CB"},
		{Coverage: "///", HeightMissing: true, Type: "///"},
		{Coverage: "BKN", HeightMissing: true},
		{Coverage: "FEW", Height: 1500, Type: "///"},
	}, m.Clouds)
	assert.Nil(t, m.Temperature)
	assert.Nil(t, m.DewPoint)
	assert.Equal(t, "hPa", m.PressureUnit)
	assert.Zero(t, m.Pressure)
	assert.Equal(t, []SeaState{
		{State: "4", Raw: "W///S4"},
		{Temperature: ptr.To(15), State: "2", Raw: "W15/S2"},
		{Temperature: ptr.To(12), WaveHeight: ptr.To(7), Raw: "W12/H7"},
		{Temperature: ptr.To(-1), Raw: "WM01///"},
	}, m.SeaStates)
	assert.Empty(t, m.Unhandled)

	expected := `Station: ENUG
Wind: Not reported (variation not reported)
Visibility: Not reported
Weather: Not reported, recent weather not reported
Clouds: Broken clouds at 800 feet (type not reported), clouds at 1,500 feet (coverage not reported), cumulonimbus (coverage and height not reported), clouds (coverage, height and type not reported), broken clouds (height not reported), few clouds at 1,500 feet (type not reported)
Ceiling: 800 feet
Temperature: Not available
Dew Point: Not available
Pressure: Not available
Sea: Surface temperature not reported, sea state moderate
Sea: Surface temperature 15°C | 59°F, sea state smooth
Sea: Surface temperature 12°C | 54°F, significant wave height 0.7 m
Sea: Surface temperature -1°C | 30°F, sea state not reported

Special Conditions:
  • Automated observation
`
	assert.Equal(t, expected, formatWithoutTime(m))
}

func TestMETARMissingValueTokensAreHandled(t *testing.T) {
	reports := []string{
		"EHJR 080725Z AUTO 15019KT //// // ///////// 09/05 Q1013 RE// W07/H5",
		"SKRH 080700Z AUTO 11006KT //// R10///// // ////// 25/// Q1013",
		"SCIC 080800Z AUTO 15007KT 120V180 //// R/////// // NCD 12/08 Q1020",
		"ESNX 080720Z AUTO 28027G41KT 1400 R30/P2000N -SN ///024/// 00/M01 Q0993",
		"MYCB 081000Z AUTO /////KT ///V/// CAVOK 13/13 A3004",
		"LIRK 080755Z AUTO 14011KT //// ///// Q////",
		"KXYZ 080755Z AUTO 14011KT 10SM VV/// //////TCU 12/10 A////",
		"PASY 080655Z AUTO M 7SM M 03/03 M",
		"EBLB 081425Z AUTO 15007KT 9999 /// 15/M04 Q1010 BLU+",
		"EGUN 081425Z 15007KT 9999 BKNTCU030 SCTCB025 15/M04 Q1010 BLACKGRN",
	}

	for _, raw := range reports {
		m := DecodeMETAR(raw, metarTestReference)
		assert.Empty(t, m.Unhandled, raw)
	}
}

func TestMETARAutomatedMissingGroupMarker(t *testing.T) {
	m := DecodeMETAR("PASY 080655Z AUTO M 7SM M 03/03 M", metarTestReference)
	assert.Equal(t, []string{"AUTO", "M"}, m.SpecialCodes)
	assert.Equal(t, "7SM", m.Visibility)
	assert.Contains(t, formatWithoutTime(m), "  • A group was not reported\n")
}

func TestMETARMilitaryColorState(t *testing.T) {
	m := DecodeMETAR("EBLB 081425Z 15007KT 9999 FEW030 15/M04 Q1010 BLACKBLU", metarTestReference)
	assert.Empty(t, m.Weather)
	assert.Equal(t, []string{"BLACKBLU"}, m.SpecialCodes)
	assert.Equal(t, "military color state blue: visibility at least 8 km and cloud base at least 2500 feet, airfield unusable for non-weather reasons (BLACKBLU)",
		describeSpecialCode("BLACKBLU"))
}

func TestMETARClouds(t *testing.T) {
	assert.Equal(t, Cloud{Coverage: "BKN", Height: 0}, parseCloud("BKN000"))
	assert.Equal(t, Cloud{Coverage: "BKN", Height: 3000, Type: "TCU"}, parseCloud("BKNTCU030"))
	assert.Equal(t, Cloud{Coverage: "CLR"}, parseCloud("CLR"))

	assert.Equal(t, "broken clouds below 100 feet", formatClouds([]Cloud{parseCloud("BKN000")}))
	assert.Equal(t, "clear", formatClouds([]Cloud{parseCloud("CLR")}))
	assert.Equal(t, "overcast at 1,200 feet (cumulonimbus)", formatClouds([]Cloud{parseCloud("OVC012CB")}))
}

func TestMETARCelsiusToFahrenheit(t *testing.T) {
	tests := map[int]int{-40: -40, -18: 0, -1: 30, 0: 32, 1: 34, 20: 68, 37: 99}
	for celsius, fahrenheit := range tests {
		assert.Equal(t, fahrenheit, CelsiusToFahrenheit(celsius), celsius)
	}
}

func TestMETARFormatGolden(t *testing.T) {
	m := DecodeMETAR("METAR KJFK 121651Z 00000KT 10SM -RA BKN000 OVC012 M01/M01 A3001", metarTestReference)
	assert.Equal(t, "METAR", m.ReportType)

	expected := `Station: KJFK
Report: Routine (METAR)
Wind: Calm
Visibility: 10 statute miles
Weather: Light rain
Clouds: Broken clouds below 100 feet, overcast at 1,200 feet
Ceiling: Below 100 feet
Flight Category: LIFR
Temperature: -1°C | 30°F
Dew Point: -1°C | 30°F
Relative Humidity: 100%
Pressure: 30.01 inHg | 1016.3 hPa
`
	assert.Equal(t, expected, formatWithoutTime(m))
}

func TestMETARTrendGroups(t *testing.T) {
	m := DecodeMETAR("METAR EGLL 171150Z 24015KT 9999 SCT030 12/08 Q1012 TEMPO FM1230 TL1400 4000 SHRA BKN014TCU BECMG AT1600 NSW NSC", metarTestReference)

	assert.Empty(t, m.Unhandled)
	assert.Equal(t, "9999", m.Visibility)
	assert.Equal(t, []Forecast{
		{
			Type:       "TEMPO",
			From:       utc(2026, time.March, 17, 12, 30),
			To:         utc(2026, time.March, 17, 14, 0),
			Visibility: "4000",
			Weather:    []string{"SHRA"},
			Clouds:     []Cloud{{Coverage: "BKN", Height: 1400, Type: "TCU"}},
			Raw:        "TEMPO FM1230 TL1400 4000 SHRA BKN014TCU",
		},
		{
			Type: "BECMG",
			At:   utc(2026, time.March, 17, 16, 0),
			NSW:  true,
			NSC:  true,
			Raw:  "BECMG AT1600 NSW NSC",
		},
	}, m.Trends)

	assert.Contains(t, formatWithoutTime(m), `
Trends:

1. Temporary 2026-03-17 12:30 UTC to 2026-03-17 14:00 UTC
   Visibility: 4,000 meters
   Weather: Rain showers
   Clouds: Broken clouds at 1,400 feet (towering cumulus)

2. Becoming at 2026-03-17 16:00 UTC
   Weather: No significant weather
   Clouds: No significant clouds
`)
}

func TestMETARTrendTimeAfterMidnight(t *testing.T) {
	m := DecodeMETAR("LFPG 172350Z 20010KT CAVOK 15/05 Q1020 BECMG TL0030 25015KT", metarTestReference)

	require.Len(t, m.Trends, 1)
	assert.Equal(t, utc(2026, time.March, 18, 0, 30), m.Trends[0].To)
	assert.Equal(t, "250", m.Trends[0].Wind.Direction)
}

func TestMETARRunwayWindShear(t *testing.T) {
	m := DecodeMETAR("OMDB 121650Z 18010KT 9999 FEW030 30/20 Q1008 WS ALL RWY WS R12L WS TKOF RWY30R", metarTestReference)

	assert.Empty(t, m.Unhandled)
	assert.Equal(t, []WindShear{
		{Type: "RWY", Phase: "ALL", Raw: "WS ALL RWY"},
		{Type: "RWY", Runway: "12L", Raw: "WS R12L"},
		{Type: "RWY", Phase: "TKOF", Runway: "30R", Raw: "WS TKOF RWY30R"},
	}, m.WindShear)
}

func TestMETARPressureGluedToNextSection(t *testing.T) {
	m := DecodeMETAR("WMKK 081130Z 20004KT 9999 VCTS FEW016CB 27/22 Q1006TEMPO FM1150 4000 TSRA", metarTestReference)

	assert.Equal(t, 1006.0, m.Pressure)
	require.Len(t, m.Trends, 1)
	assert.Equal(t, []string{"TSRA"}, m.Trends[0].Weather)
	assert.Empty(t, m.Unhandled)
}

func TestMETARStationPressure(t *testing.T) {
	for _, raw := range []string{
		"SLCB 081200Z 00000KT 6000 BKN008 11/11 QFE 774.3 BR",
		"SLCB 081200Z 00000KT 6000 BKN008 11/11 QFE774.3 BR",
	} {
		m := DecodeMETAR(raw, metarTestReference)
		assert.Equal(t, 774.3, m.StationPressure, raw)
		assert.Equal(t, []string{"BR"}, m.Weather, raw)
		assert.Empty(t, m.Unhandled, raw)
		assert.Contains(t, formatWithoutTime(m), "Station Pressure: 774.3 hPa\n")
	}
}

func TestMETARRemarksWithoutBody(t *testing.T) {
	assert.NotPanics(t, func() { DecodeMETAR("KXYZ RMK AO2", metarTestReference) })
	m := DecodeMETAR("KXYZ 121651Z RMK AO2", metarTestReference)
	assert.Len(t, m.Remarks, 1)
}

func TestMETARSecondPressureGroup(t *testing.T) {
	m := DecodeMETAR("LGAV 080750Z 36012KT 9999 FEW030 15/08 Q1020 A3015", metarTestReference)
	assert.Equal(t, 1020.0, m.Pressure)
	assert.Equal(t, "hPa", m.PressureUnit)
	assert.Equal(t, PressureReading{Value: 30.15, Unit: "inHg"}, m.SecondPressure)
	assert.Empty(t, m.Unhandled)
	assert.Contains(t, formatWithoutTime(m), "Pressure: 1020.0 hPa | 30.12 inHg\nSecond Pressure Group: 30.15 inHg | 1021.0 hPa\n")

	missing := DecodeMETAR("LGAV 080750Z 36012KT 9999 FEW030 15/08 Q1020 A////", metarTestReference)
	assert.Equal(t, PressureReading{Unit: "inHg"}, missing.SecondPressure)
	assert.Contains(t, formatWithoutTime(missing), "Second Pressure Group: Not available\n")
}

func TestMETARSecondUndirectedVisibilityIsMinimum(t *testing.T) {
	m := DecodeMETAR("EDDV 080720Z AUTO 09007KT 9999 2500 BR NCD 03/00 Q1017 NOSIG", metarTestReference)
	assert.Equal(t, "9999", m.Visibility)
	assert.Equal(t, "2500", m.MinVisibility)
	assert.Empty(t, m.Unhandled)
	assert.Contains(t, formatWithoutTime(m), "Visibility: Unlimited visibility (greater than 10 kilometers)\nMinimum Visibility: 2,500 meters\n")
}

func TestMETARVerticalVisibilityAtSurface(t *testing.T) {
	m := DecodeMETAR("KMWN 080658Z 28077G88KT 0SM -SN FZFG BLSN VV000 M18/M18 RMK PK WND 28098/07", metarTestReference)
	require.NotNil(t, m.VertVis)
	assert.Equal(t, 0, *m.VertVis)
	assert.False(t, m.VertVisMissing)

	formatted := formatWithoutTime(m)
	assert.Contains(t, formatted, "Vertical Visibility: Below 100 feet\n")
	assert.Contains(t, formatted, "Ceiling: Below 100 feet\nFlight Category: LIFR\n")

	missing := DecodeMETAR("KXYZ 080755Z AUTO 14011KT 1/4SM FG VV/// 12/10 A3001", metarTestReference)
	assert.Nil(t, missing.VertVis)
	assert.True(t, missing.VertVisMissing)
	assert.Contains(t, formatWithoutTime(missing), "Vertical Visibility: Not reported\n")

	absent := DecodeMETAR("KXYZ 080755Z 14011KT 10SM CLR 12/10 A3001", metarTestReference)
	assert.Nil(t, absent.VertVis)
}

func TestMETARSingleDigitTemperatures(t *testing.T) {
	tests := []struct {
		raw         string
		temperature *int
		dewPoint    *int
	}{
		{"DNKA 080700Z 03004KT CAVOK 24/5 Q1014 NOSIG", ptr.To(24), ptr.To(5)},
		{"VOHY 080830Z 07005KT 6000 NSC 36/1 Q1010 NOSIG", ptr.To(36), ptr.To(1)},
		{"KXYZ 080755Z 14011KT 10SM CLR 5/M3 A3001", ptr.To(5), ptr.To(-3)},
		{"KXYZ 080755Z 14011KT 10SM CLR M5/ A3001", ptr.To(-5), nil},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			m := DecodeMETAR(tt.raw, metarTestReference)
			assert.Equal(t, tt.temperature, m.Temperature)
			assert.Equal(t, tt.dewPoint, m.DewPoint)
			assert.Empty(t, m.Unhandled)
		})
	}
}

func TestMETARMissingStatuteMileVisibility(t *testing.T) {
	m := DecodeMETAR("KAXA 080755Z AUTO 31008KT ////SM CLR M02/M04 A3012 RMK AO2 VISNO", metarTestReference)
	assert.Equal(t, "////SM", m.Visibility)
	assert.Empty(t, m.Unhandled)
	assert.Contains(t, formatWithoutTime(m), "Visibility: Not reported\n")
}
