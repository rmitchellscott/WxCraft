package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessRemarks(t *testing.T) {
	tests := []struct {
		name    string
		remarks string
		want    []Remark
	}{
		{
			name:    "peak wind with minutes only and with hour",
			remarks: "PK WND 28045/15 PK WND 18052/0658",
			want: []Remark{
				{"PK WND 28045/15", "peak wind 280° at 45 knots at 15 minutes past the hour"},
				{"PK WND 18052/0658", "peak wind 180° at 52 knots at 06:58"},
			},
		},
		{
			name:    "wind shift with frontal passage",
			remarks: "WSHFT 0709 FROPA WSHFT 30",
			want: []Remark{
				{"WSHFT 0709 FROPA", "wind shift at 07:09 due to frontal passage"},
				{"WSHFT 30", "wind shift at 30 minutes past the hour"},
			},
		},
		{
			name:    "ceiling groups",
			remarks: "CIG 007V010 CIG 005 RWY 6L CIG 019 RWY15R CIG030 CHINO RWY11 VISNO S",
			want: []Remark{
				{"CIG 007V010", "variable ceiling between 700 and 1000 feet"},
				{"CIG 005 RWY 6L", "ceiling 500 feet at runway 6L"},
				{"CIG 019 RWY15R", "ceiling 1900 feet at runway 15R"},
				{"CIG030", "ceiling 3000 feet"},
				{"CHINO RWY11", "cloud height not available at runway 11"},
				{"VISNO S", "visibility not available south"},
			},
		},
		{
			name:    "begin and end times",
			remarks: "RAB15E30 TSB0159E30 FZRAB05 RAE1955 SHRAB10E25RAB40",
			want: []Remark{
				{"RAB15E30", "rain began at 15 minutes past the hour, ended at 30 minutes past the hour"},
				{"TSB0159E30", "thunderstorm began at 01:59, ended at 30 minutes past the hour"},
				{"FZRAB05", "freezing rain began at 5 minutes past the hour"},
				{"RAE1955", "rain ended at 19:55"},
				{"SHRAB10E25RAB40", "rain showers began at 10 minutes past the hour, ended at 25 minutes past the hour; rain began at 40 minutes past the hour"},
			},
		},
		{
			name:    "additive numeric groups",
			remarks: "10142 20012 401001015 52018 60125 60000 6//// 70125 T0011 T01330111 T1007////",
			want: []Remark{
				{"10142", "6-hour maximum temperature 14.2°C"},
				{"20012", "6-hour minimum temperature 1.2°C"},
				{"401001015", "24-hour temperature range: max 10.0°C (50.0°F), min -1.5°C (29.3°F)"},
				{"52018", "pressure tendency: increasing steadily, 1.8 hPa change"},
				{"60125", "3- or 6-hour precipitation: 1.25 inches"},
				{"60000", "3- or 6-hour precipitation: trace"},
				{"6////", "3- or 6-hour precipitation: indeterminate amount"},
				{"70125", "24-hour precipitation: 1.25 inches"},
				{"T0011", "temperature 1.1°C"},
				{"T01330111", "temperature 13.3°C, dew point 11.1°C"},
				{"T1007////", "temperature -0.7°C, dew point not available"},
			},
		},
		{
			name:    "five-digit group starting with 3 is not a pressure change",
			remarks: "30021",
			want: []Remark{
				{"30021", "unknown remark code"},
			},
		},
		{
			name:    "snow and ice groups",
			remarks: "I1000 I3/// I6012 4/012 933021 931011 98096 SNINCR 2/10",
			want: []Remark{
				{"I1000", "1-hour ice accretion: 0.00 inches"},
				{"I3///", "3-hour ice accretion not available"},
				{"I6012", "6-hour ice accretion: 0.12 inches"},
				{"4/012", "snow depth: 12 inches"},
				{"933021", "water equivalent of snow on ground: 2.1 inches"},
				{"931011", "snowfall in the last 6 hours: 1.1 inches"},
				{"98096", "sunshine duration on the previous day: 96 minutes"},
				{"SNINCR 2/10", "snow increasing rapidly: 2 inches in the past hour, 10 inches on ground"},
			},
		},
		{
			name:    "sensor status and report flags",
			remarks: "PWINO PNO RVRNO VISNO CHINO TSNO FZRANO $ NOSPECI FIRST LAST",
			want: []Remark{
				{"PWINO", "precipitation identifier sensor not available"},
				{"PNO", "precipitation amount not available"},
				{"RVRNO", "runway visual range not available"},
				{"VISNO", "visibility at second location not available"},
				{"CHINO", "cloud height at second location not available"},
				{"TSNO", "thunderstorm information not available"},
				{"FZRANO", "freezing rain information not available"},
				{"$", "weather observing equipment requires maintenance"},
				{"NOSPECI", "no SPECI reports are taken at this station"},
				{"FIRST", "first observation after a break in coverage"},
				{"LAST", "last observation before a break in coverage"},
			},
		},
		{
			name:    "lightning",
			remarks: "FRQ LTGICCG OHD OCNL LTGICCC S-SW AND DSNT SE-SW LTG DSNT NE LTG DSNT ALQS",
			want: []Remark{
				{"FRQ LTGICCG OHD", "frequent lightning (in-cloud, cloud-to-ground) overhead"},
				{"OCNL LTGICCC S-SW AND DSNT SE-SW", "occasional lightning (in-cloud, cloud-to-cloud) south through southwest and distant southeast through southwest"},
				{"LTG DSNT NE", "lightning distant northeast"},
				{"LTG DSNT ALQS", "lightning distant in all quadrants"},
			},
		},
		{
			name:    "thunderstorm, cloud and virga location and movement",
			remarks: "TS NE MOV SE CB DSNT SE-SW MOV NE VIRGA DSNT NE VIRGA",
			want: []Remark{
				{"TS NE MOV SE", "thunderstorm northeast moving southeast"},
				{"CB DSNT SE-SW MOV NE", "cumulonimbus distant southeast through southwest moving northeast"},
				{"VIRGA DSNT NE", "virga distant northeast"},
				{"VIRGA", "virga"},
			},
		},
		{
			name:    "visibility groups",
			remarks: "TWR VIS 1 1/2 SFC VIS 10 VIS 1/2V2 VIS 3/8V1 1/4 VIS 3/4 RWY11 VIS 1200 RWY22",
			want: []Remark{
				{"TWR VIS 1 1/2", "tower visibility 1 1/2 statute miles"},
				{"SFC VIS 10", "surface visibility 10 statute miles"},
				{"VIS 1/2V2", "variable visibility from 1/2 statute miles to 2 statute miles"},
				{"VIS 3/8V1 1/4", "variable visibility from 3/8 statute miles to 1 1/4 statute miles"},
				{"VIS 3/4 RWY11", "visibility 3/4 statute miles at runway 11"},
				{"VIS 1200 RWY22", "visibility 1200 meters at runway 22"},
			},
		},
		{
			name:    "aircraft mishap and variable sky condition",
			remarks: "ACFT MSHP BKN014 V OVC",
			want: []Remark{
				{"ACFT MSHP", "aircraft mishap"},
				{"BKN014 V OVC", "variable sky condition: broken clouds at 1400 feet varying with overcast"},
			},
		},
		{
			name:    "Canadian layer opacity and snow on ground",
			remarks: "SC4AC2 SC TR SOG 02",
			want: []Remark{
				{"SC4AC2", "cloud layer opacity: stratocumulus 4/8, altocumulus 2/8"},
				{"SC TR", "trace of stratocumulus"},
				{"SOG 02", "snow on ground: 2 cm"},
			},
		},
		{
			name:    "Japanese cloud layers",
			remarks: "1ST008 4CU014 7SC050",
			want: []Remark{
				{"1ST008", "1/8 stratus at 800 feet"},
				{"4CU014", "4/8 cumulus at 1400 feet"},
				{"7SC050", "7/8 stratocumulus at 5000 feet"},
			},
		},
		{
			name:    "Russian QFE and cloud base",
			remarks: "QFE741/0988 QFE734 QBB180",
			want: []Remark{
				{"QFE741/0988", "QFE 741 mmHg (988 hPa)"},
				{"QFE734", "QFE 734 mmHg"},
				{"QBB180", "cloud base 180 meters"},
			},
		},
		{
			name:    "Mongolian QFE with relative humidity",
			remarks: "QFE641.0 24",
			want: []Remark{
				{"QFE641.0 24", "QFE 641.0 mmHg, relative humidity 24%"},
			},
		},
		{
			name:    "Canadian automatic station groups",
			remarks: "SOG 04 8005 PCPN 2.2MM PAST HR DP MISG ICE INTMT VIS VRB 1 3/4-3 CIG VRB 9-15 LTNG DIST ALQDS WND ESTD",
			want: []Remark{
				{"SOG 04", "snow on ground: 4 cm"},
				{"8005", "3-hour pressure tendency: decreasing, then decreasing more rapidly, 0.5 hPa change"},
				{"PCPN 2.2MM PAST HR", "2.2 mm of precipitation in the past hour"},
				{"DP MISG", "dew point data missing"},
				{"ICE INTMT", "intermittent icing detected"},
				{"VIS VRB 1 3/4-3", "variable visibility from 1 3/4 statute miles to 3 statute miles"},
				{"CIG VRB 9-15", "variable ceiling between 900 and 1500 feet"},
				{"LTNG DIST ALQDS", "lightning distant in all quadrants"},
				{"WND ESTD", "wind estimated"},
			},
		},
		{
			name:    "Canadian snowfall since the last main synoptic hour",
			remarks: "SN8 /S07/ /S10 AFT 23Z/ /S05 AFT 0300 UTC/ SLP829",
			want: []Remark{
				{"SN8", "cloud layer opacity: snow 8/8"},
				{"/S07/", "snowfall since the last main synoptic hour: 7 cm"},
				{"/S10 AFT 23Z/", "snowfall since 23:00 UTC: 10 cm"},
				{"/S05 AFT 0300 UTC/", "snowfall since 03:00 UTC: 5 cm"},
				{"SLP829", "sea level pressure 982.9 hPa"},
			},
		},
		{
			name:    "numbers inside plain language stay with the text",
			remarks: "A01 WINDS UN-RELIABLE 350 TO 45 DEGREES RW 14/32 EDGE LIGHTS OTS BKN BKN080 SLP012 27",
			want: []Remark{
				{"A01", "automated station without precipitation sensor"},
				{"WINDS UN-RELIABLE 350 TO 45 DEGREES RW 14/32 EDGE LIGHTS OTS BKN", "plain-language remark"},
				{"BKN080", "unknown remark code"},
				{"SLP012", "sea level pressure 1001.2 hPa"},
				{"27", "unknown remark code"},
			},
		},
		{
			name:    "military color states",
			remarks: "BLU YLO1 BLACKRED",
			want: []Remark{
				{"BLU", "military color state blue: visibility at least 8 km and cloud base at least 2500 feet"},
				{"YLO1", "military color state yellow 1: visibility at least 2.5 km and cloud base at least 500 feet"},
				{"BLACKRED", "military color state red: visibility less than 0.8 km or cloud base less than 200 feet; airfield not usable for reasons other than weather"},
			},
		},
		{
			name:    "winds at other locations",
			remarks: "WIND 670FT 27018G29KT WIND THR03 04007KT RWY18C 22003KT 190V250 MID 36002MPS END R02/36002MPS",
			want: []Remark{
				{"WIND 670FT 27018G29KT", "wind at 670 feet from 270° at 18 knots, gusting 29 knots"},
				{"WIND THR03 04007KT", "wind at runway 03 threshold from 040° at 7 knots"},
				{"RWY18C 22003KT 190V250", "runway 18C wind from 220° at 3 knots, varying between 190° and 250°"},
				{"MID 36002MPS", "mid-runway wind from 360° at 2 m/s"},
				{"END R02/36002MPS", "runway 02 end wind from 360° at 2 m/s"},
			},
		},
		{
			name:    "cloud type group",
			remarks: "8/5//",
			want: []Remark{
				{"8/5//", "cloud types: low stratocumulus, middle not observable, high not observable"},
			},
		},
		{
			name:    "plain language is kept together",
			remarks: "AO2 UPCOMING TAXIWY CLOSURES. CHECK NOTAMS SLP012",
			want: []Remark{
				{"AO2", "automated station with precipitation sensor"},
				{"UPCOMING TAXIWY CLOSURES. CHECK NOTAMS", "plain-language remark"},
				{"SLP012", "sea level pressure 1001.2 hPa"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, processRemarks(strings.Fields(tt.remarks)))
		})
	}
}
