package main

import (
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestParseStatuteMiles(t *testing.T) {
	tests := []struct {
		visibility string
		want       statuteMiles
		ok         bool
	}{
		{"10SM", statuteMiles{miles: 10}, true},
		{"P6SM", statuteMiles{miles: 6, moreThan: true}, true},
		{"M1/4SM", statuteMiles{miles: 0.25, lessThan: true}, true},
		{"1 1/2SM", statuteMiles{miles: 1.5}, true},
		{"3/4SM", statuteMiles{miles: 0.75}, true},
		{"0SM", statuteMiles{miles: 0}, true},
		{"1600", statuteMiles{miles: 1600 / metersPerStatuteMile}, true},
		{"4000NE", statuteMiles{miles: 4000 / metersPerStatuteMile}, true},
		{"19KM", statuteMiles{miles: 19000 / metersPerStatuteMile}, true},
		{"0000", statuteMiles{miles: 50 / metersPerStatuteMile, lessThan: true}, true},
		{"9999", statuteMiles{miles: 10000 / metersPerStatuteMile, moreThan: true}, true},
		{"9999NDV", statuteMiles{miles: 10000 / metersPerStatuteMile, moreThan: true}, true},
		{"CAVOK", statuteMiles{miles: 10000 / metersPerStatuteMile, moreThan: true}, true},
		{"////", statuteMiles{}, false},
		{"////SM", statuteMiles{}, false},
		{"", statuteMiles{}, false},
	}
	for _, tt := range tests {
		got, ok := parseStatuteMiles(tt.visibility)
		assert.Equal(t, tt.ok, ok, tt.visibility)
		assert.InDelta(t, tt.want.miles, got.miles, 1e-9, tt.visibility)
		assert.Equal(t, tt.want.lessThan, got.lessThan, tt.visibility)
		assert.Equal(t, tt.want.moreThan, got.moreThan, tt.visibility)
	}
}

func TestMETARCeilingAndFlightCategory(t *testing.T) {
	tests := []struct {
		body         string
		ceilingFeet  int
		noCeiling    bool
		ceilingKnown bool
		category     string
	}{
		{"10SM FEW050", 0, true, true, "VFR"},
		{"10SM CLR", 0, true, true, "VFR"},
		{"CAVOK", 0, true, true, "VFR"},
		{"9999 NCD", 0, true, true, "VFR"},
		{"P6SM BKN031", 3100, false, true, "VFR"},
		{"10SM FEW005 BKN030 OVC050", 3000, false, true, "MVFR"},
		{"5SM SCT020", 0, true, true, "MVFR"},
		{"5000 BR SCT020", 0, true, true, "MVFR"},
		{"4800 BR SCT020", 0, true, true, "IFR"},
		{"1700 BR SCT020", 0, true, true, "IFR"},
		{"1600 BR SCT020", 0, true, true, "LIFR"},
		{"12KM -RA FEW020", 0, true, true, "VFR"},
		{"7KM BR SCT020", 0, true, true, "MVFR"},
		{"10SM OVC009", 900, false, true, "IFR"},
		{"2 1/2SM BR BKN012", 1200, false, true, "IFR"},
		{"1SM BR OVC005", 500, false, true, "IFR"},
		{"10SM OVC004", 400, false, true, "LIFR"},
		{"M1/4SM FG VV002", 200, false, true, "LIFR"},
		{"0800 FG SCT001 VV003", 300, false, true, "LIFR"},
		{"1/2SM FG BKN000", 0, false, true, "LIFR"},
		{"//// BKN050", 5000, false, true, ""},
		{"//// OVC002", 200, false, true, "LIFR"},
		{"1/2SM FG //////", 0, false, false, "LIFR"},
		{"10SM ///015 BKN030", 0, false, false, ""},
		{"10SM BKN///", 0, false, false, ""},
		{"10SM VV///", 0, false, false, ""},
		{"10SM", 0, false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			m := DecodeMETAR("KXYZ 080755Z 14011KT "+tt.body+" 12/10 A3001", metarTestReference)
			require.Empty(t, m.Unhandled)
			derived := deriveMETAR(m)
			assert.Equal(t, tt.ceilingKnown, derived.ceilingKnown)
			assert.Equal(t, ceiling{feet: tt.ceilingFeet, none: tt.noCeiling}, derived.ceiling)
			assert.Equal(t, tt.category, derived.flightCategory)
		})
	}
}

func TestRelativeHumidity(t *testing.T) {
	assert.InDelta(t, 52.5, relativeHumidity(20, 10), 0.05)
	assert.InDelta(t, 100, relativeHumidity(-5, -5), 1e-9)
	assert.InDelta(t, 100, relativeHumidity(10, 10.2), 1e-9)
	assert.InDelta(t, 21.8, relativeHumidity(35, 10), 0.05)
}

func TestDensityAltitude(t *testing.T) {
	assert.InDelta(t, 8289, densityAltitude(1656/0.3048, 30.12, 30), 0.5)
	assert.InDelta(t, 0, densityAltitude(0, 29.92, 15), 1e-9)
}

func TestMETARTenthsTemperatureAndDensityAltitude(t *testing.T) {
	m := DecodeMETAR("KDEN 081853Z 21012KT 10SM FEW120 SCT200 30/M01 A3012 RMK AO2 SLP120 T03001006", metarTestReference)
	m.SiteInfo = SiteInfo{Name: "Denver Intl", State: "CO", Country: "United States", Elevation: ptr.To(1656)}

	derived := deriveMETAR(m)
	require.NotNil(t, derived.temperature)
	assert.InDelta(t, 30.0, *derived.temperature, 1e-9)
	assert.InDelta(t, -0.6, *derived.dewPoint, 1e-9)
	assert.True(t, derived.tenths)
	require.NotNil(t, derived.densityAltitude)
	assert.Equal(t, 8289, *derived.densityAltitude)

	expected := `Station: KDEN (Denver Intl, CO, United States)
Wind: From 210° at 12 knots
Visibility: 10 statute miles
Clouds: Few clouds at 12,000 feet, scattered clouds at 20,000 feet
Ceiling: None
Flight Category: VFR
Temperature: 30.0°C | 86°F
Dew Point: -0.6°C | 31°F
Relative Humidity: 14%
Pressure: 30.12 inHg | 1020.0 hPa
Density Altitude: 8,289 feet

Remarks:
  AO2: Automated station with precipitation sensor
  SLP120: Sea level pressure 1012.0 hPa
  T03001006: Temperature 30.0°C, dew point -0.6°C
`
	assert.Equal(t, expected, formatWithoutTime(m))
}

func TestMETARDensityAltitudeNeedsElevation(t *testing.T) {
	m := DecodeMETAR("LFPG 081200Z 27008KT CAVOK 18/09 Q1020 NOSIG", metarTestReference)
	assert.Nil(t, deriveMETAR(m).densityAltitude)

	m.SiteInfo.Elevation = ptr.To(unknownStationElevation)
	assert.Nil(t, deriveMETAR(m).densityAltitude)

	m.SiteInfo.Elevation = ptr.To(119)
	require.NotNil(t, deriveMETAR(m).densityAltitude)
	assert.Equal(t, 595, *deriveMETAR(m).densityAltitude)

	m.Pressure, m.PressureUnit = 0, "hPa"
	assert.Nil(t, deriveMETAR(m).densityAltitude)
}

func TestMETARCAVOKInMetersGolden(t *testing.T) {
	m := DecodeMETAR("METAR LFPG 081200Z 27008KT CAVOK 18/09 Q1020 NOSIG", metarTestReference)
	m.SiteInfo = SiteInfo{Name: "Paris/Charles de Gaulle", Country: "France", Elevation: ptr.To(119)}

	expected := `Station: LFPG (Paris/Charles de Gaulle, France)
Report: Routine (METAR)
Wind: From 270° at 8 knots
Visibility: Greater than 10 km
Ceiling: None
Flight Category: VFR
Temperature: 18°C | 64°F
Dew Point: 9°C | 48°F
Relative Humidity: 56%
Pressure: 1020.0 hPa | 30.12 inHg
Density Altitude: 595 feet

Special Conditions:
  • Ceiling and visibility OK
  • No significant changes expected
`
	assert.Equal(t, expected, formatWithoutTime(m))
}

const tempoProbTAF = "TAF KXYZ 080530Z 0806/0906 18010KT P6SM SCT040 TEMPO 0808/0812 3SM -SHRA BKN015 " +
	"FM081400 20012KT 5SM BR OVC008 PROB30 0816/0820 1/2SM FG VV002 BECMG 0822/0824 P6SM BKN030 " +
	"FM090200 VRB05KT P6SM SKC"

func TestDecodeTAF_PeriodsInEffect(t *testing.T) {
	tests := []struct {
		reference time.Time
		inEffect  []bool
	}{
		{utc(2025, time.March, 8, 5, 30), []bool{false, false, false, false, false, false}},
		{utc(2025, time.March, 8, 6, 0), []bool{true, false, false, false, false, false}},
		{utc(2025, time.March, 8, 9, 0), []bool{true, true, false, false, false, false}},
		{utc(2025, time.March, 8, 14, 0), []bool{false, false, true, false, false, false}},
		{utc(2025, time.March, 8, 17, 0), []bool{false, false, true, true, false, false}},
		{utc(2025, time.March, 8, 23, 0), []bool{false, false, true, false, true, false}},
		{utc(2025, time.March, 9, 1, 0), []bool{false, false, true, false, true, false}},
		{utc(2025, time.March, 9, 2, 0), []bool{false, false, false, false, false, true}},
		{utc(2025, time.March, 9, 6, 0), []bool{false, false, false, false, false, false}},
	}
	for _, tt := range tests {
		t.Run(tt.reference.Format(time.RFC3339), func(t *testing.T) {
			taf := DecodeTAF(tempoProbTAF, tt.reference)
			require.Len(t, taf.Forecasts, len(tt.inEffect))
			for i, forecast := range taf.Forecasts {
				assert.Equal(t, tt.inEffect[i], forecast.InEffect, forecast.Raw)
			}
		})
	}
}

func TestFormatTAF_TempoAndProbPeriodsGolden(t *testing.T) {
	color.NoColor = true
	taf := DecodeTAF(tempoProbTAF, utc(2025, time.March, 8, 9, 0))
	taf.Time = time.Time{}

	expected := `Station: KXYZ
Valid: 2025-03-08 06:00 UTC to 2025-03-09 06:00 UTC

Forecast Periods:

1. Base Forecast 2025-03-08 06:00 UTC to 2025-03-08 14:00 UTC (in effect)
   Wind: From 180° at 10 knots
   Visibility: Greater than 6 statute miles
   Clouds: Scattered clouds at 4,000 feet
   Ceiling: None
   Flight Category: VFR

2. Temporary 2025-03-08 08:00 UTC to 2025-03-08 12:00 UTC (in effect)
   Visibility: 3 statute miles
   Weather: Light rain showers
   Clouds: Broken clouds at 1,500 feet
   Ceiling: 1,500 feet
   Flight Category: MVFR

3. From 2025-03-08 14:00 UTC to 2025-03-09 02:00 UTC
   Wind: From 200° at 12 knots
   Visibility: 5 statute miles
   Weather: Mist
   Clouds: Overcast at 800 feet
   Ceiling: 800 feet
   Flight Category: IFR

4. 30% Probability 2025-03-08 16:00 UTC to 2025-03-08 20:00 UTC
   Visibility: 1/2 statute miles
   Vertical Visibility: 200 feet
   Weather: Fog
   Ceiling: 200 feet
   Flight Category: LIFR

5. Becoming 2025-03-08 22:00 UTC to 2025-03-09 00:00 UTC
   Visibility: Greater than 6 statute miles
   Clouds: Broken clouds at 3,000 feet
   Ceiling: 3,000 feet
   Flight Category: MVFR

6. From 2025-03-09 02:00 UTC to 2025-03-09 06:00 UTC
   Wind: Variable at 5 knots
   Visibility: Greater than 6 statute miles
   Clouds: Sky clear
   Ceiling: None
   Flight Category: VFR
`
	assert.Equal(t, expected, FormatTAF(taf))
}

func TestPeriodSkies_ChangePeriodsKeepOmittedGroups(t *testing.T) {
	taf := DecodeTAF("TAF EGXX 080500Z 0806/0906 24010KT 9999 BKN008 TEMPO 0808/0812 6000 BR "+
		"BECMG 0812/0814 SCT030 TEMPO 0816/0818 BKN012 BECMG 0820/0822 CAVOK TEMPO 0822/0902 4000 RA "+
		"FM090300 27010KT 3000 BR", tafTestNow)
	require.Len(t, taf.Forecasts, 7)

	skies := periodSkies(taf.Forecasts)
	var categories []string
	for _, sky := range skies {
		categories = append(categories, sky.flightCategory())
	}
	assert.Equal(t, []string{"IFR", "IFR", "VFR", "MVFR", "VFR", "IFR", ""}, categories)

	ceiling, ok := skies[1].ceiling()
	assert.True(t, ok)
	assert.Equal(t, 800, ceiling.feet, "TEMPO without clouds keeps the base BKN008")
	assert.Equal(t, "9999", skies[3].visibility, "TEMPO without visibility keeps 9999")
}
