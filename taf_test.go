package main

import (
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func utc(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
}

var tafTestNow = utc(2025, time.March, 8, 9, 0)

func TestDecodeTAF_StatuteMileVisibility(t *testing.T) {
	taf := DecodeTAF("TAF CZUM 080747Z 0808/0820 VRB03KT 1 1/2SM -SN OVC006 TEMPO 0808/0814 5SM -SN FM081400 32008KT P6SM SCT015 TEMPO 0814/0819 1/2SM FG BECMG 0815/0817 M1/4SM FG", tafTestNow)

	require.Len(t, taf.Forecasts, 5)
	assert.Equal(t, "1 1/2SM", taf.Forecasts[0].Visibility)
	assert.Equal(t, "5SM", taf.Forecasts[1].Visibility)
	assert.Equal(t, "P6SM", taf.Forecasts[2].Visibility)
	assert.Equal(t, "1/2SM", taf.Forecasts[3].Visibility)
	assert.Equal(t, "M1/4SM", taf.Forecasts[4].Visibility)
	for _, forecast := range taf.Forecasts {
		assert.Empty(t, forecast.Unhandled, forecast.Raw)
	}
}

func TestDecodeTAF_CAVOKAndNilSignificantGroups(t *testing.T) {
	taf := DecodeTAF("TAF EGSS 080743Z 0807/0912 10008KT 7000 NSC BECMG 0807/0809 CAVOK TEMPO 0900/0909 6000 -RA BECMG 0909/0911 9999 NSW", tafTestNow)

	require.Len(t, taf.Forecasts, 4)
	assert.True(t, taf.Forecasts[0].NSC)
	assert.Equal(t, "7000", taf.Forecasts[0].Visibility)
	assert.True(t, taf.Forecasts[1].CAVOK)
	assert.Equal(t, "CAVOK", taf.Forecasts[1].Visibility)
	assert.Equal(t, []string{"-RA"}, taf.Forecasts[2].Weather)
	assert.True(t, taf.Forecasts[3].NSW)
	assert.Empty(t, taf.Forecasts[3].Weather)
}

func TestDecodeTAF_MilitaryGroups(t *testing.T) {
	taf := DecodeTAF("TAF KBLV 080800Z 0808/0914 35012G20KT 9999 BKN015 620204 QNH2998INS BECMG 0810/0811 34012KT 9999 FEW250 WS020/24040KT 520009 QNH3003INS TX11/0821Z TN02/0812Z TXM03/0900Z TNM15/0913Z", tafTestNow)

	require.Len(t, taf.Forecasts, 2)
	base := taf.Forecasts[0]
	assert.Equal(t, 29.98, base.Pressure)
	assert.Equal(t, "inHg", base.PressureUnit)
	assert.Equal(t, []HazardLayer{{Intensity: "2", BaseFeet: 2000, ThicknessFeet: 4000, Raw: "620204"}}, base.Icing)

	becoming := taf.Forecasts[1]
	assert.Equal(t, 30.03, becoming.Pressure)
	assert.Equal(t, []HazardLayer{{Intensity: "2", BaseFeet: 0, ThicknessFeet: 9000, Raw: "520009"}}, becoming.Turbulence)
	require.Len(t, becoming.WindShear, 1)
	assert.Equal(t, WindShear{
		Type:     "ALT",
		Altitude: 20,
		Wind:     Wind{Direction: "240", Speed: ptr.To(40), Unit: "KT"},
		Raw:      "WS020/24040KT",
	}, becoming.WindShear[0])
	assert.Empty(t, becoming.Unhandled)

	assert.Equal(t, []TemperatureForecast{
		{Kind: "MAX", Celsius: 11, At: utc(2025, time.March, 8, 21, 0), Raw: "TX11/0821Z"},
		{Kind: "MIN", Celsius: 2, At: utc(2025, time.March, 8, 12, 0), Raw: "TN02/0812Z"},
		{Kind: "MAX", Celsius: -3, At: utc(2025, time.March, 9, 0, 0), Raw: "TXM03/0900Z"},
		{Kind: "MIN", Celsius: -15, At: utc(2025, time.March, 9, 13, 0), Raw: "TNM15/0913Z"},
	}, taf.Temperatures)
}

func TestDecodeTAF_MilitaryPlainLanguageTrailer(t *testing.T) {
	taf := DecodeTAF("TAF AMD KGUR 080745Z 0807/0911 VRB06KT 9999 FEW020 QNH3006INS TX07/0823Z LAST NO AMDS AFT 0807 NXT 1005 TNM15/0813Z", tafTestNow)

	require.Len(t, taf.Forecasts, 1)
	assert.Equal(t, "9999", taf.Forecasts[0].Visibility)
	assert.Empty(t, taf.Forecasts[0].Unhandled)
	assert.Equal(t, "LAST NO AMDS AFT 0807 NXT 1005", taf.Remarks)
	require.Len(t, taf.Temperatures, 2)
	assert.Equal(t, -15, taf.Temperatures[1].Celsius)
}

func TestDecodeTAF_MilitaryWindNoteKeepsForecastWind(t *testing.T) {
	taf := DecodeTAF("TAF KVBG 072315Z 0723/0905 34012G20KT 9999 SCT015 QNH3010INS WND 02006KT AFT 0801 BECMG 0802/0803 36009KT 9999 SKC WND 270V360", tafTestNow)

	require.Len(t, taf.Forecasts, 2)
	assert.Equal(t, "340", taf.Forecasts[0].Wind.Direction)
	assert.Equal(t, []string{"WND", "02006KT", "AFT", "0801"}, taf.Forecasts[0].Unhandled)
	assert.Equal(t, "BECMG", taf.Forecasts[1].Type)
	assert.Empty(t, taf.Forecasts[1].WindVariation)
	assert.Equal(t, []string{"WND", "270V360"}, taf.Forecasts[1].Unhandled)
}

func TestFormatTAF_ShowsProbabilityOnEveryPeriodType(t *testing.T) {
	color.NoColor = true
	taf := DecodeTAF("TAF LFTH 080748Z 0807/0903 09015KT 9999 SCT018 PROB30 TEMPO 0809/0812 BKN014 PROB40 0814/0816 4000 RA", tafTestNow)

	formatted := FormatTAF(taf)
	assert.Contains(t, formatted, "30% Probability, Temporary")
	assert.Contains(t, formatted, "40% Probability 2025")
}

func TestDecodeTAF_InterGroups(t *testing.T) {
	taf := DecodeTAF("TAF AMD YSSY 080547Z 0806/0912 07015KT 9999 -SHRA BKN030 FM080900 07010KT 9999 -SHRA BKN025 INTER 0809/0900 VRB15KT 4000 SHRA BKN010 PROB30 INTER 0812/0818 2000 TSRA", tafTestNow)

	require.Len(t, taf.Forecasts, 4)
	inter := taf.Forecasts[2]
	assert.Equal(t, "INTER", inter.Type)
	assert.Equal(t, 0, inter.Probability)
	assert.Equal(t, utc(2025, time.March, 8, 9, 0), inter.From)
	assert.Equal(t, utc(2025, time.March, 9, 0, 0), inter.To)
	assert.Equal(t, "4000", inter.Visibility)

	probInter := taf.Forecasts[3]
	assert.Equal(t, "INTER", probInter.Type)
	assert.Equal(t, 30, probInter.Probability)
	assert.Equal(t, "PROB30 INTER 0812/0818 2000 TSRA", probInter.Raw)
}

func TestDecodeTAF_AmendedAndCorrectedHeaders(t *testing.T) {
	amended := DecodeTAF("TAF AMD EGJA 080746Z 0807/0815 13013KT CAVOK", tafTestNow)
	assert.Equal(t, "EGJA", amended.Station)
	assert.True(t, amended.Amended)
	assert.False(t, amended.Corrected)
	assert.Equal(t, utc(2025, time.March, 8, 7, 46), amended.Time)

	corrected := DecodeTAF("TAF COR EGPD 080713Z 0807/0906 10005KT 3000 BR BKN003", tafTestNow)
	assert.Equal(t, "EGPD", corrected.Station)
	assert.True(t, corrected.Corrected)
	assert.Equal(t, utc(2025, time.March, 8, 7, 0), corrected.ValidFrom)
	assert.Equal(t, utc(2025, time.March, 9, 6, 0), corrected.ValidTo)
}

func TestDecodeTAF_NilAndCancelled(t *testing.T) {
	nilReport := DecodeTAF("TAF KXYZ 081100Z NIL", tafTestNow)
	assert.Equal(t, "KXYZ", nilReport.Station)
	assert.True(t, nilReport.Nil)
	assert.Empty(t, nilReport.Forecasts)

	cancelled := DecodeTAF("TAF AMD KXYZ 081100Z 0812/0912 CNL", tafTestNow)
	assert.True(t, cancelled.Cancelled)
	assert.Equal(t, utc(2025, time.March, 8, 12, 0), cancelled.ValidFrom)
	assert.Empty(t, cancelled.Forecasts)

	assert.NotPanics(t, func() { DecodeTAF("TAF", tafTestNow) })
	assert.NotPanics(t, func() { DecodeTAF("TAF AMD", tafTestNow) })
	assert.NotPanics(t, func() { DecodeTAF("", tafTestNow) })
}

func TestDecodeTAF_StationStartingWithFM(t *testing.T) {
	taf := DecodeTAF("TAF FMEE 080500Z 0806/0912 12010KT 9999 FEW020 FM081800 14005KT CAVOK", tafTestNow)

	assert.Equal(t, "FMEE", taf.Station)
	require.Len(t, taf.Forecasts, 2)
	assert.Equal(t, "BASE", taf.Forecasts[0].Type)
	assert.Equal(t, "120", taf.Forecasts[0].Wind.Direction)
	assert.Equal(t, utc(2025, time.March, 8, 18, 0), taf.Forecasts[0].To)
	assert.Equal(t, "FM", taf.Forecasts[1].Type)
	assert.Equal(t, utc(2025, time.March, 8, 18, 0), taf.Forecasts[1].From)
	assert.Equal(t, utc(2025, time.March, 9, 12, 0), taf.Forecasts[1].To)
}

func TestDecodeTAF_SplitFromGroup(t *testing.T) {
	taf := DecodeTAF("TAF FZIC 080500Z 0806/0912 23008KT 6000 BKN014 FM 090000 00000KT FG BKN002", tafTestNow)

	require.Len(t, taf.Forecasts, 2)
	assert.Equal(t, "FM", taf.Forecasts[1].Type)
	assert.Equal(t, utc(2025, time.March, 9, 0, 0), taf.Forecasts[1].From)
	assert.Empty(t, taf.Forecasts[1].Unhandled)
}

func TestDecodeTAF_ProbabilityTempo(t *testing.T) {
	taf := DecodeTAF("TAF AMD LFTH 080748Z 0807/0903 09015KT 9999 SCT018 PROB30 TEMPO 0809/0812 BKN014 PROB40TEMPO 0812/0814 4000 RA PROB30 0819/0821 TSRA", tafTestNow)

	require.Len(t, taf.Forecasts, 4)

	probTempo := taf.Forecasts[1]
	assert.Equal(t, "TEMPO", probTempo.Type)
	assert.Equal(t, 30, probTempo.Probability)
	assert.Equal(t, utc(2025, time.March, 8, 9, 0), probTempo.From)
	assert.Equal(t, utc(2025, time.March, 8, 12, 0), probTempo.To)
	assert.Equal(t, []Cloud{{Coverage: "BKN", Height: 1400}}, probTempo.Clouds)

	joined := taf.Forecasts[2]
	assert.Equal(t, "TEMPO", joined.Type)
	assert.Equal(t, 40, joined.Probability)
	assert.Equal(t, "4000", joined.Visibility)

	prob := taf.Forecasts[3]
	assert.Equal(t, "PROB", prob.Type)
	assert.Equal(t, 30, prob.Probability)
	assert.Equal(t, []string{"TSRA"}, prob.Weather)
}

func TestDecodeTAF_RemarksStopForecastParsing(t *testing.T) {
	taf := DecodeTAF("TAF CYGQ 080740Z 0808/0820 27005KT P6SM SKC TEMPO 0818/0820 5SM -SN BKN020 RMK FCST BASED ON AUTO OBS. NXT FCST BY 081400Z", tafTestNow)

	assert.Equal(t, "FCST BASED ON AUTO OBS. NXT FCST BY 081400Z", taf.Remarks)
	require.Len(t, taf.Forecasts, 2)
	tempo := taf.Forecasts[1]
	assert.Equal(t, []string{"-SN"}, tempo.Weather)
	assert.Empty(t, tempo.Unhandled)
	assert.Equal(t, "TEMPO 0818/0820 5SM -SN BKN020", tempo.Raw)
}

func TestDecodeTAF_PeriodsCrossMidnightAndHour24(t *testing.T) {
	taf := DecodeTAF("TAF KHOU 082320Z 0900/1006 15006KT P6SM BKN020 TEMPO 0918/0924 3SM BR FM092330 20005KT P6SM SKC BECMG 1002/1004 VRB03KT", utc(2025, time.March, 8, 23, 30))

	assert.Equal(t, utc(2025, time.March, 8, 23, 20), taf.Time)
	assert.Equal(t, utc(2025, time.March, 9, 0, 0), taf.ValidFrom)
	assert.Equal(t, utc(2025, time.March, 10, 6, 0), taf.ValidTo)

	require.Len(t, taf.Forecasts, 4)
	assert.Equal(t, utc(2025, time.March, 9, 18, 0), taf.Forecasts[1].From)
	assert.Equal(t, utc(2025, time.March, 10, 0, 0), taf.Forecasts[1].To)
	assert.Equal(t, utc(2025, time.March, 9, 23, 30), taf.Forecasts[2].From)
	assert.Equal(t, utc(2025, time.March, 10, 6, 0), taf.Forecasts[2].To)
	assert.Equal(t, utc(2025, time.March, 9, 23, 30), taf.Forecasts[0].To)
	assert.Equal(t, utc(2025, time.March, 10, 2, 0), taf.Forecasts[3].From)
	assert.Equal(t, utc(2025, time.March, 10, 4, 0), taf.Forecasts[3].To)
}

func TestDecodeTAF_PeriodsCrossMonthAndYearEnd(t *testing.T) {
	raw := "TAF EGLL 311700Z 3118/0124 24010KT 9999 SCT030 BECMG 3122/3124 30015KT TEMPO 0100/0106 7000 -RA FM010900 32010KT CAVOK TX08/3118Z TN02/0106Z"

	for _, now := range []time.Time{
		utc(2025, time.December, 31, 17, 5),
		utc(2026, time.January, 1, 2, 0),
		utc(2026, time.January, 2, 0, 0),
	} {
		taf := DecodeTAF(raw, now)

		assert.Equal(t, utc(2025, time.December, 31, 17, 0), taf.Time, now)
		assert.Equal(t, utc(2025, time.December, 31, 18, 0), taf.ValidFrom, now)
		assert.Equal(t, utc(2026, time.January, 2, 0, 0), taf.ValidTo, now)

		require.Len(t, taf.Forecasts, 4)
		assert.Equal(t, utc(2025, time.December, 31, 22, 0), taf.Forecasts[1].From, now)
		assert.Equal(t, utc(2026, time.January, 1, 0, 0), taf.Forecasts[1].To, now)
		assert.Equal(t, utc(2026, time.January, 1, 0, 0), taf.Forecasts[2].From, now)
		assert.Equal(t, utc(2026, time.January, 1, 6, 0), taf.Forecasts[2].To, now)
		assert.Equal(t, utc(2026, time.January, 1, 9, 0), taf.Forecasts[3].From, now)
		assert.Equal(t, utc(2026, time.January, 2, 0, 0), taf.Forecasts[3].To, now)
		assert.Equal(t, utc(2026, time.January, 1, 9, 0), taf.Forecasts[0].To, now)

		assert.Equal(t, utc(2025, time.December, 31, 18, 0), taf.Temperatures[0].At, now)
		assert.Equal(t, utc(2026, time.January, 1, 6, 0), taf.Temperatures[1].At, now)
	}
}

func TestDecodeTAF_ShortMonthEnd(t *testing.T) {
	taf := DecodeTAF("TAF KXYZ 282330Z 0100/0206 18005KT P6SM SKC FM011200 20010KT P6SM SKC", utc(2025, time.March, 1, 0, 10))

	assert.Equal(t, utc(2025, time.February, 28, 23, 30), taf.Time)
	assert.Equal(t, utc(2025, time.March, 1, 0, 0), taf.ValidFrom)
	assert.Equal(t, utc(2025, time.March, 2, 6, 0), taf.ValidTo)
	assert.Equal(t, utc(2025, time.March, 1, 12, 0), taf.Forecasts[1].From)
}

func TestDecodeTAF_NoIssueTimeAnchorsOnValidity(t *testing.T) {
	taf := DecodeTAF("TAF KNGP 3123/0123 16010KT 1600 BR BKN005 FM010400 17008KT 4800 BR SCT008", utc(2026, time.January, 1, 1, 0))

	assert.True(t, taf.Time.IsZero())
	assert.Equal(t, utc(2025, time.December, 31, 23, 0), taf.ValidFrom)
	assert.Equal(t, utc(2026, time.January, 1, 23, 0), taf.ValidTo)
	assert.Equal(t, utc(2026, time.January, 1, 4, 0), taf.Forecasts[1].From)
}

func TestParseTime_ResolvesMonthNearestReference(t *testing.T) {
	cases := []struct {
		raw       string
		reference time.Time
		want      time.Time
	}{
		{"081151Z", utc(2025, time.March, 8, 12, 0), utc(2025, time.March, 8, 11, 51)},
		{"312355Z", utc(2026, time.January, 1, 0, 5), utc(2025, time.December, 31, 23, 55)},
		{"010005Z", utc(2025, time.December, 31, 23, 55), utc(2026, time.January, 1, 0, 5)},
	}
	for _, tc := range cases {
		got, err := parseTime(tc.raw, tc.reference)
		require.NoError(t, err, tc.raw)
		assert.Equal(t, tc.want, got, tc.raw)
	}

	_, err := parseTime("321200Z", tafTestNow)
	assert.Error(t, err)
}

func TestFormatTAF_MilitaryGroupsFlagsAndRemarks(t *testing.T) {
	color.NoColor = true
	taf := DecodeTAF("TAF AMD KBLV 080800Z 0808/0914 35012G20KT 9999 BKN015 620204 QNH2998INS BECMG 0810/0811 34012KT 9999 FEW250 WS020/24040KT 520009 QNH3003INS TX11/0821Z TN02/0812Z LAST NO AMDS AFT 0807 NXT 1005", tafTestNow)
	taf.Time = time.Time{}

	expected := `Station: KBLV
Report: Amended
Valid: 2025-03-08 08:00 UTC to 2025-03-09 14:00 UTC

Forecast Periods:

1. Base Forecast 2025-03-08 08:00 UTC to 2025-03-09 14:00 UTC (in effect)
   Wind: From 350° at 12 knots, gusting to 20 knots
   Visibility: Unlimited visibility (greater than 10 kilometers)
   Clouds: Broken clouds at 1,500 feet
   Ceiling: 1,500 feet
   Flight Category: MVFR
   Pressure: 29.98 inHg | 1015.2 hPa
   Icing: Light icing in cloud from 2,000 feet to 6,000 feet

2. Becoming 2025-03-08 10:00 UTC to 2025-03-08 11:00 UTC
   Wind: From 340° at 12 knots
   Visibility: Unlimited visibility (greater than 10 kilometers)
   Clouds: Few clouds at 25,000 feet
   Ceiling: None
   Flight Category: VFR
   Pressure: 30.03 inHg | 1016.9 hPa
   Wind Shear: At 2,000 feet: From 240° at 40 knots
   Turbulence: Occasional moderate turbulence in clear air from the surface to 9,000 feet

Temperatures:
  Maximum 11°C | 52°F at 2025-03-08 21:00 UTC
  Minimum 2°C | 36°F at 2025-03-08 12:00 UTC

Remarks:
  LAST NO AMDS AFT 0807 NXT 1005
`
	assert.Equal(t, expected, FormatTAF(taf))
}

func TestFormatTAF_NilSignificantGroupsAndCancelled(t *testing.T) {
	color.NoColor = true

	significant := FormatTAF(DecodeTAF("TAF EGSS 080743Z 0807/0912 10008KT 7000 NSC BECMG 0909/0911 9999 NSW", tafTestNow))
	assert.Contains(t, significant, "   Clouds: No significant clouds\n")
	assert.Contains(t, significant, "   Weather: No significant weather\n")

	cancelled := DecodeTAF("TAF AMD KXYZ 081100Z 0812/0912 CNL", tafTestNow)
	cancelled.Time = time.Time{}
	assert.Equal(t, `Station: KXYZ
Report: Amended, cancelled
Valid: 2025-03-08 12:00 UTC to 2025-03-09 12:00 UTC
`, FormatTAF(cancelled))
}
