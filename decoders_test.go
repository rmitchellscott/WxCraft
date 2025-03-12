package main

import (
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/rmitchellscott/WxCraft/testdata"
	"github.com/stretchr/testify/assert"
)

func decodeMETARList(t *testing.T) iter.Seq2[string, METAR] {
	return func(yield func(string, METAR) bool) {
		scanner := testdata.METAR(t)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "//") {
				continue
			}
			line := strings.TrimSpace(scanner.Text())
			if !yield(line, DecodeMETAR(line)) {
				return
			}
		}
	}
}

func TestDecodeMETAR_stationCode(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		assert.Equal(t, fields[0], metar.SiteInfo.Name, line, "station code did not match")
	}
}

func TestDecodeMETAR_time(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		if fields[1] != "COR" {
			got := metar.Time.Format("021504") + "Z"
			assert.Equal(t, fields[1], got, line, "time did not match")
		}
	}
}

func TestDecodeMETAR_remarks(t *testing.T) {
	t.Parallel()

	var unknownRemarks []string
	var failedRemarkCount int

	for line, metar := range decodeMETARList(t) {
		var unknown []string
		for _, rmk := range metar.Remarks {
			if rmk.Description == "unknown remark code" {
				unknown = append(unknown, rmk.Raw)
				unknownRemarks = append(unknownRemarks, rmk.Raw)
			}
		}

		if len(unknown) != 0 {
			failedRemarkCount++
			t.Errorf("Unknown remarks:\nMETAR   = %s\nRemarks = %v", line, unknown)
		}
	}

	t.Run("unknown remark count", func(t *testing.T) {
		slices.Sort(unknownRemarks)
		unknownRemarks = slices.Compact(unknownRemarks)
		assert.Empty(t, len(unknownRemarks))
	})

	t.Run("metars with failed remarks", func(t *testing.T) {
		assert.Zero(t, failedRemarkCount)
	})
}

func TestDecodeMETAR_visibility(t *testing.T) {
	t.Parallel()
	for line, metar := range decodeMETARList(t) {
		fields := strings.Fields(line)
		for _, field := range fields[1:] {
			if strings.HasSuffix(field, "SM") {
				assert.Equal(t, field, metar.Visibility)
			}
		}
	}
}
