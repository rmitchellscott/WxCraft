package main

import (
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

// formatWithoutTime renders a METAR with the observation time cleared, since the
// formatter prints the age relative to time.Now
func formatWithoutTime(m METAR) string {
	color.NoColor = true
	m.Time = time.Time{}
	return FormatMETAR(m)
}

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
			m := DecodeMETAR(tt.raw)
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
	m := DecodeMETAR("UWKE 080747Z 33005MPS 9999 OVC008 M02/M02 Q1010 " +
		"R24/290050 R88/CLRD// R06/CLRD62 R07/32//50 R02/////// R/SNOCLO " +
		"R24/P2000N R04/1200V1800FT/U R10///// NOSIG")

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
Temperature: -2°C | 28°F
Dew Point: -2°C | 28°F
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
			m := DecodeMETAR("KXYZ 121651Z " + tt.raw + " 10SM CLR 20/10 A3001")
			assert.Equal(t, tt.wind, m.Wind)
			assert.Equal(t, tt.expected, formatWind(m.Wind))
			assert.Empty(t, m.Unhandled)
		})
	}

	assert.Equal(t, "", formatWind(Wind{}))
}

func TestMETARWindShearSpeed(t *testing.T) {
	m := DecodeMETAR("KXYZ 121651Z 18010KT 10SM CLR 20/10 A3001 WS020/27045G55KT")
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
	m := DecodeMETAR("LIRF 121650Z 18005KT 4000 1500NE BR SCT010 12/11 Q1018")
	assert.Equal(t, "4000", m.Visibility)
	assert.Equal(t, "1500NE", m.MinVisibility)
	assert.Equal(t, []string{"BR"}, m.Weather)
	assert.Empty(t, m.Unhandled)
	assert.Contains(t, formatWithoutTime(m),
		"Visibility: 4,000 meters\nMinimum Visibility: 1,500 meters in the NE direction\n")
}

func TestMETARMissingValues(t *testing.T) {
	m := DecodeMETAR("ENUG 080750Z AUTO /////KT ///V/// //// // BKN008/// ///015 ///CB " +
		"///////// BKN/// FEW015/// ///// Q//// W///S4 W15/S2 W12/H7 WM01/// RE//")

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
		m := DecodeMETAR(raw)
		assert.Empty(t, m.Unhandled, raw)
	}
}

func TestMETARAutomatedMissingGroupMarker(t *testing.T) {
	m := DecodeMETAR("PASY 080655Z AUTO M 7SM M 03/03 M")
	assert.Equal(t, []string{"AUTO", "M"}, m.SpecialCodes)
	assert.Equal(t, "7SM", m.Visibility)
	assert.Contains(t, formatWithoutTime(m), "  • A group was not reported\n")
}

func TestMETARMilitaryColorState(t *testing.T) {
	m := DecodeMETAR("EBLB 081425Z 15007KT 9999 FEW030 15/M04 Q1010 BLACKBLU")
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
	m := DecodeMETAR("METAR KJFK 121651Z 00000KT 10SM -RA BKN000 OVC012 M01/M01 A3001")
	assert.Equal(t, "METAR", m.ReportType)

	expected := `Station: KJFK
Wind: Calm
Visibility: 10 statute miles
Weather: Light rain
Clouds: Broken clouds below 100 feet, overcast at 1,200 feet
Temperature: -1°C | 30°F
Dew Point: -1°C | 30°F
Pressure: 30.01 inHg | 1016.3 hPa
`
	assert.Equal(t, expected, formatWithoutTime(m))
}
