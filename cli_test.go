package main

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rmitchellscott/WxCraft/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitReports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []report
	}{
		{
			name:  "metar",
			input: "KJFK 110154Z 09007KT 10SM FEW040 BKN250 12/01 A3013 RMK AO2",
			want: []report{
				{Kind: metarReport, Station: "KJFK", Raw: "KJFK 110154Z 09007KT 10SM FEW040 BKN250 12/01 A3013 RMK AO2"},
			},
		},
		{
			name:  "metar with tempo trend",
			input: "EGLL 171150Z 24015KT 9999 SCT030 12/08 Q1012 TEMPO 4000 SHRA",
			want: []report{
				{Kind: metarReport, Station: "EGLL", Raw: "EGLL 171150Z 24015KT 9999 SCT030 12/08 Q1012 TEMPO 4000 SHRA"},
			},
		},
		{
			name:  "metar with becmg trend and keyword",
			input: "METAR LFPG 171200Z 20010KT CAVOK 15/05 Q1020 BECMG 25015KT",
			want: []report{
				{Kind: metarReport, Station: "LFPG", Raw: "METAR LFPG 171200Z 20010KT CAVOK 15/05 Q1020 BECMG 25015KT"},
			},
		},
		{
			name:  "speci",
			input: "SPECI KORD 171212Z 27020G30KT 1SM +TSRA OVC010 20/18 A2990",
			want: []report{
				{Kind: metarReport, Station: "KORD", Raw: "SPECI KORD 171212Z 27020G30KT 1SM +TSRA OVC010 20/18 A2990"},
			},
		},
		{
			name:  "taf with keyword",
			input: "TAF KBOS 110547Z 1106/1212 14012KT 4SM -RA BR OVC008",
			want: []report{
				{Kind: tafReport, Station: "KBOS", Raw: "TAF KBOS 110547Z 1106/1212 14012KT 4SM -RA BR OVC008"},
			},
		},
		{
			name:  "taf amd",
			input: "TAF AMD LFTH 080748Z 0807/0903 09015KT 9999 SCT018",
			want: []report{
				{Kind: tafReport, Station: "LFTH", Raw: "TAF AMD LFTH 080748Z 0807/0903 09015KT 9999 SCT018"},
			},
		},
		{
			name:  "taf without keyword",
			input: "KFST 080742Z 0808/0906 30017G28KT 4SM HZ BKN200 FM081500 29019G26KT P6SM SCT250",
			want: []report{
				{Kind: tafReport, Station: "KFST", Raw: "KFST 080742Z 0808/0906 30017G28KT 4SM HZ BKN200 FM081500 29019G26KT P6SM SCT250"},
			},
		},
		{
			name:  "taf without issue time",
			input: "KFST 0808/0906 30017G28KT 4SM HZ BKN200",
			want: []report{
				{Kind: tafReport, Station: "KFST", Raw: "KFST 0808/0906 30017G28KT 4SM HZ BKN200"},
			},
		},
		{
			name:  "station starting with FM",
			input: "TAF FMEE 171100Z 1712/1818 12010KT 9999 FEW020",
			want: []report{
				{Kind: tafReport, Station: "FMEE", Raw: "TAF FMEE 171100Z 1712/1818 12010KT 9999 FEW020"},
			},
		},
		{
			name: "multi-line taf with indented continuation lines",
			input: "TAF KJFK 171120Z 1712/1818 27012KT P6SM FEW050\n" +
				"      FM171800 29015G25KT P6SM SCT060\n" +
				"      TEMPO 1720/1722 BKN040",
			want: []report{
				{Kind: tafReport, Station: "KJFK", Raw: "TAF KJFK 171120Z 1712/1818 27012KT P6SM FEW050\n" +
					"      FM171800 29015G25KT P6SM SCT060\n" +
					"      TEMPO 1720/1722 BKN040"},
			},
		},
		{
			name:  "keyword on its own line",
			input: "TAF AMD\nKBOS 110547Z 1106/1212 14012KT 4SM -RA BR OVC008",
			want: []report{
				{Kind: tafReport, Station: "KBOS", Raw: "TAF AMD KBOS 110547Z 1106/1212 14012KT 4SM -RA BR OVC008"},
			},
		},
		{
			name: "several reports",
			input: "KJFK 171151Z 27012KT 10SM FEW050 15/03 A3002\n" +
				"KLGA 171151Z 28010KT 10SM SCT060 15/02 A3001 RMK AO2\n" +
				"\n" +
				"TAF KJFK 171120Z 1712/1818 27012KT P6SM FEW050\n" +
				"  FM171800 29015G25KT P6SM SCT060\n" +
				"KEWR 171151Z 27011KT 10SM FEW055 16/02 A3002",
			want: []report{
				{Kind: metarReport, Station: "KJFK", Raw: "KJFK 171151Z 27012KT 10SM FEW050 15/03 A3002"},
				{Kind: metarReport, Station: "KLGA", Raw: "KLGA 171151Z 28010KT 10SM SCT060 15/02 A3001 RMK AO2"},
				{Kind: tafReport, Station: "KJFK", Raw: "TAF KJFK 171120Z 1712/1818 27012KT P6SM FEW050\n  FM171800 29015G25KT P6SM SCT060"},
				{Kind: metarReport, Station: "KEWR", Raw: "KEWR 171151Z 27011KT 10SM FEW055 16/02 A3002"},
			},
		},
		{
			name:  "equals terminators",
			input: "METAR EGLL 171150Z 24015KT 9999 SCT030 12/08 Q1012 NOSIG= METAR EGKK 171150Z 23012KT 9999 FEW025 13/07 Q1013=",
			want: []report{
				{Kind: metarReport, Station: "EGLL", Raw: "METAR EGLL 171150Z 24015KT 9999 SCT030 12/08 Q1012 NOSIG"},
				{Kind: metarReport, Station: "EGKK", Raw: "METAR EGKK 171150Z 23012KT 9999 FEW025 13/07 Q1013"},
			},
		},
		{
			name:  "keyword only",
			input: "TAF",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitReports(tt.input))
		})
	}
}

// TestSplitReports_corpus checks that every corpus report is one report of the right kind
func TestSplitReports_corpus(t *testing.T) {
	t.Parallel()

	corpora := []struct {
		name    string
		scanner func(*testing.T) *bufio.Scanner
		want    reportKind
	}{
		{"metar", testdata.METAR, metarReport},
		{"taf", testdata.TAF, tafReport},
	}

	for _, corpus := range corpora {
		t.Run(corpus.name, func(t *testing.T) {
			var misclassified []string
			scanner := corpus.scanner(t)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || line == "---" {
					continue
				}
				reports := splitReports(line)
				if len(reports) != 1 || reports[0].Kind != corpus.want {
					misclassified = append(misclassified, line)
				}
			}
			assert.Empty(t, misclassified)
		})
	}
}

func TestStationResolver(t *testing.T) {
	t.Parallel()

	resolver := stationResolver{
		nearest: func(radiusMiles float64) (string, error) {
			return fmt.Sprintf("NEAR%.0f", radiusMiles), nil
		},
		byZipcode: func(zipcode string, radiusMiles float64) (string, error) {
			if zipcode == "00000" {
				return "", errors.New("zipcode not found")
			}
			return "ZIP:" + zipcode, nil
		},
	}

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "KJFK", want: "KJFK"},
		{input: " egll\n", want: "EGLL"},
		{input: "K1G4", want: "K1G4"},
		{input: "AUTO", want: "NEAR25"},
		{input: "auto\n", want: "NEAR25"},
		{input: "80202", want: "ZIP:80202"},
		{input: "80202-1234", want: "ZIP:80202-1234"},
		{input: "00000", wantErr: true},
		{input: "JFK", wantErr: true},
		{input: "KJFKX", wantErr: true},
		{input: "KJ-K", wantErr: true},
		{input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := resolver.resolve(tt.input, 25)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
