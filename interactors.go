package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
)

type reportKind int

const (
	metarReport reportKind = iota
	tafReport
)

// report is one METAR or TAF found in piped input
type report struct {
	Kind    reportKind
	Station string
	Raw     string
}

var (
	reportStationRegex  = regexp.MustCompile(`^[A-Z][A-Z0-9]{3}$`)
	reportIssueRegex    = regexp.MustCompile(`^\d{6}Z$`)
	reportValidityRegex = regexp.MustCompile(`^\d{4}/\d{4}$`)
	stationCodeRegex    = regexp.MustCompile(`^[A-Z0-9]{4}$`)
)

var reportHeaderKeywords = []string{"TAF", "METAR", "SPECI", "AMD", "COR", "RTD"}

// readPipedStdin returns the trimmed stdin contents when stdin is a pipe or file, and "" for a terminal
func readPipedStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("error reading stdin: %w", err)
	}
	return strings.TrimSpace(string(input)), nil
}

// splitReports splits pasted text into reports. A report starts at a line that opens with a
// report header and runs until the next header or an "=" terminator, so the indented
// continuation lines of a multi-line TAF stay with their report.
func splitReports(input string) []report {
	var reports []report
	var current, pendingKeywords []string

	flush := func() {
		if len(current) > 0 {
			reports = append(reports, classifyReport(strings.Join(current, "\n")))
			current = nil
		}
	}

	for _, line := range strings.Split(input, "\n") {
		segments := strings.Split(line, "=")
		for segmentIndex, segment := range segments {
			fields := strings.Fields(segment)
			switch {
			case len(fields) == 0:
			case isKeywordOnly(fields):
				flush()
				pendingKeywords = append(pendingKeywords, fields...)
			case len(pendingKeywords) > 0:
				current = []string{strings.Join(pendingKeywords, " ") + " " + strings.TrimSpace(segment)}
				pendingKeywords = nil
			case isReportStart(fields):
				flush()
				current = []string{strings.TrimSpace(segment)}
			default:
				current = append(current, strings.TrimRight(segment, " \t\r"))
			}
			if segmentIndex < len(segments)-1 {
				flush()
			}
		}
	}
	flush()

	return reports
}

func isKeywordOnly(fields []string) bool {
	for _, field := range fields {
		if !slices.Contains(reportHeaderKeywords, field) {
			return false
		}
	}
	return true
}

func isReportStart(fields []string) bool {
	if slices.Contains(reportHeaderKeywords, fields[0]) {
		return true
	}
	return len(fields) > 1 && reportStationRegex.MatchString(fields[0]) &&
		(reportIssueRegex.MatchString(fields[1]) || reportValidityRegex.MatchString(fields[1]))
}

// classifyReport decides METAR or TAF from the header alone: an explicit TAF, METAR or SPECI
// keyword wins, otherwise a ddhh/ddhh validity period after the station (with or without an
// issue time before it) marks a TAF. Trend groups such as TEMPO occur in both report types.
func classifyReport(raw string) report {
	fields := strings.Fields(raw)
	kind := metarReport
	hasTypeKeyword := false

	i := 0
	for ; i < len(fields) && slices.Contains(reportHeaderKeywords, fields[i]); i++ {
		switch fields[i] {
		case "TAF":
			kind = tafReport
			hasTypeKeyword = true
		case "METAR", "SPECI":
			kind = metarReport
			hasTypeKeyword = true
		}
	}

	var station string
	if i < len(fields) {
		station = fields[i]
	}

	if !hasTypeKeyword {
		after := fields[min(i+1, len(fields)):]
		if len(after) > 0 && reportIssueRegex.MatchString(after[0]) {
			after = after[1:]
		}
		if len(after) > 0 && reportValidityRegex.MatchString(after[0]) {
			kind = tafReport
		}
	}

	return report{Kind: kind, Station: station, Raw: raw}
}

// stationResolver turns user input (ICAO code, US zipcode or AUTO) into an ICAO station code
type stationResolver struct {
	nearest   func(radiusMiles float64) (string, error)
	byZipcode func(zipcode string, radiusMiles float64) (string, error)
}

var networkStationResolver = stationResolver{
	nearest:   ProcessAutoCommand,
	byZipcode: ProcessZipcode,
}

func (r stationResolver) resolve(input string, radiusMiles float64) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(input))
	switch {
	case code == "AUTO":
		return r.nearest(radiusMiles)
	case zipRegex.MatchString(code):
		return r.byZipcode(code, radiusMiles)
	case !stationCodeRegex.MatchString(code):
		return "", fmt.Errorf("invalid station code %q: must be a 4-character ICAO code, a US zipcode, or AUTO", input)
	}
	return code, nil
}

// promptForStationCode prompts the user for a station code
func promptForStationCode() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ICAO airport code (e.g., KJFK, EGLL), US zipcode, or 'AUTO' for nearest airport: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}
	return input, nil
}
