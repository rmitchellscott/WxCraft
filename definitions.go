package main

import (
	"regexp"
	"time"
)

// Common weather phenomena mapping used across the application
var weatherCodes = map[string]WeatherCode{
	"WS":  {Description: "wind shear", Position: 1},
	"VC":  {Description: "in the vicinity", Position: 3},
	"+":   {Description: "heavy", Position: 0},
	"-":   {Description: "light", Position: 0},
	"RE":  {Description: "recent", Position: 0},
	"MI":  {Description: "shallow", Position: 0},
	"PR":  {Description: "partial", Position: 0},
	"BC":  {Description: "patches", Position: 0},
	"DR":  {Description: "low drifting", Position: 0},
	"BL":  {Description: "blowing", Position: 0},
	"SH":  {Description: "showers", Position: 2},
	"TS":  {Description: "thunderstorm", Position: 1},
	"FZ":  {Description: "freezing", Position: 0},
	"DZ":  {Description: "drizzle", Position: 1},
	"RA":  {Description: "rain", Position: 1},
	"SN":  {Description: "snow", Position: 1},
	"SG":  {Description: "snow grains", Position: 1},
	"IC":  {Description: "ice crystals", Position: 1},
	"PL":  {Description: "ice pellets", Position: 1},
	"GR":  {Description: "hail", Position: 1},
	"GS":  {Description: "small hail", Position: 1},
	"UP":  {Description: "unknown precipitation", Position: 1},
	"BR":  {Description: "mist", Position: 1},
	"FG":  {Description: "fog", Position: 1},
	"FU":  {Description: "smoke", Position: 1},
	"VA":  {Description: "volcanic ash", Position: 1},
	"DU":  {Description: "widespread dust", Position: 1},
	"SA":  {Description: "sand", Position: 1},
	"HZ":  {Description: "haze", Position: 1},
	"PY":  {Description: "spray", Position: 1},
	"PO":  {Description: "dust whirls", Position: 1},
	"SQ":  {Description: "squalls", Position: 1},
	"FC":  {Description: "funnel cloud", Position: 1},
	"+FC": {Description: "tornado/waterspout", Position: 1},
	"SS":  {Description: "sandstorm", Position: 1},
	"DS":  {Description: "duststorm", Position: 1},
}

// Common cloud coverage mapping
var cloudCoverage = map[string]string{
	"SKC": "sky clear",
	"CLR": "clear",
	"FEW": "few clouds",
	"SCT": "scattered clouds",
	"BKN": "broken clouds",
	"OVC": "overcast",
}

// Common cloud type mapping
var cloudTypes = map[string]string{
	"CB":  "cumulonimbus",
	"TCU": "towering cumulus",
}

// Special aerodrome conditions
var specialConditions = map[string]string{
	"NOSIG": "no significant changes expected",
	"AUTO":  "automated observation",
	"COR":   "corrected report",
	"CCA":   "corrected report",
	"NSC":   "no significant clouds",
	"NCD":   "no clouds detected",
	"CAVOK": "ceiling and visibility OK",
	"RTD":   "routine delayed (late) observation",
	"NIL":   "missing report",
	"M":     "a group was not reported",
}

// Military airfield color states, best to worst
var militaryColorStates = map[string]string{
	"BLU":  "blue: visibility at least 8 km and cloud base at least 2500 feet",
	"WHT":  "white: visibility at least 5 km and cloud base at least 1500 feet",
	"GRN":  "green: visibility at least 3.7 km and cloud base at least 700 feet",
	"YLO":  "yellow: visibility at least 1.6 km and cloud base at least 300 feet",
	"YLO1": "yellow 1: visibility at least 2.5 km and cloud base at least 500 feet",
	"YLO2": "yellow 2: visibility at least 1.6 km and cloud base at least 300 feet",
	"AMB":  "amber: visibility at least 0.8 km and cloud base at least 200 feet",
	"RED":  "red: visibility less than 0.8 km or cloud base less than 200 feet",
}

// Runway state group deposit types (WMO code table 0919)
var runwayDeposits = map[string]string{
	"0": "clear and dry",
	"1": "damp",
	"2": "wet or water patches",
	"3": "rime or frost",
	"4": "dry snow",
	"5": "wet snow",
	"6": "slush",
	"7": "ice",
	"8": "compacted or rolled snow",
	"9": "frozen ruts or ridges",
}

// Runway state group contamination extent (WMO code table 0519)
var runwayContaminationExtents = map[string]string{
	"1": "10% or less covered",
	"2": "11-25% covered",
	"5": "26-50% covered",
	"9": "51-100% covered",
}

// Sea state codes (WMO code table 3700)
var seaStates = map[string]string{
	"0": "calm (glassy)",
	"1": "calm (rippled)",
	"2": "smooth",
	"3": "slight",
	"4": "moderate",
	"5": "rough",
	"6": "very rough",
	"7": "high",
	"8": "very high",
	"9": "phenomenal",
}

var reportTypes = map[string]string{
	"METAR": "Routine (METAR)",
	"SPECI": "Special (SPECI)",
}

var periodTypes = map[string]string{
	"BASE":  "Base Forecast",
	"FM":    "From",
	"TEMPO": "Temporary",
	"BECMG": "Becoming",
	"INTER": "Intermittent",
}

// Icing intensity and type for the military 6IchhhT group (WMO code table 1734)
var icingIntensities = map[string]string{
	"0": "no icing",
	"1": "light icing",
	"2": "light icing in cloud",
	"3": "light icing in precipitation",
	"4": "moderate icing",
	"5": "moderate icing in cloud",
	"6": "moderate icing in precipitation",
	"7": "severe icing",
	"8": "severe icing in cloud",
	"9": "severe icing in precipitation",
}

// Turbulence intensity for the military 5Bhhht group (WMO code table 1738)
var turbulenceIntensities = map[string]string{
	"0": "no turbulence",
	"1": "light turbulence",
	"2": "occasional moderate turbulence in clear air",
	"3": "frequent moderate turbulence in clear air",
	"4": "occasional moderate turbulence in cloud",
	"5": "frequent moderate turbulence in cloud",
	"6": "occasional severe turbulence in clear air",
	"7": "frequent severe turbulence in clear air",
	"8": "occasional severe turbulence in cloud",
	"9": "frequent severe turbulence in cloud",
}

// Words that open the plain-language notes US military TAFs append after the forecast groups,
// such as "LAST NO AMDS AFT 0807 NXT 1005", whose 4-digit times would otherwise read as visibility
var tafPlainLanguageStarts = map[string]bool{
	"LAST":      true,
	"NXT":       true,
	"NEXT":      true,
	"AUTOMATED": true,
	"AMD":       true,
	"COR":       true,
}

// Commonly used regular expressions
var (
	timeRegex         = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
	windRegex         = regexp.MustCompile(`^(VRB|\d{3}|///)(\d{2,3}|//)(G(\d{2,3}))?KT$|^(0+)(G\d{2})?KT$`)
	windRegexMPS      = regexp.MustCompile(`^(VRB|\d{3}|///)(\d{2,3}|//)(G(\d{2,3}))?MPS$|^(0+)(G\d{2})?MPS$`)
	windVarRegex      = regexp.MustCompile(`^(\d{3}|///)V(\d{3}|///)$`)
	windShearAltRegex = regexp.MustCompile(`^WS(\d{3})/(\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	windShearRwyRegex = regexp.MustCompile(`^WS (?:(TKOF|LDG|ALL) RWY(\d{2}[LCR]?)?|R(?:WY)?(\d{2}[LCR]?))$`)
	visRegexSM        = regexp.MustCompile(`^[MP]?\d+(?:/\d+)?SM$`)
	visRegexKM        = regexp.MustCompile(`^(\d{1,2})KM$`)
	visRegexNum       = regexp.MustCompile(`^\d{4}$`)
	visRegexDir       = regexp.MustCompile(`^(\d{4})([NESW]{1,2})$`)
	cloudRegex        = regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC|///)(\d{3}|///)?(CB|TCU|///)?$|^(FEW|SCT|BKN|OVC)(CB|TCU)(\d{3})$`)
	tempRegex         = regexp.MustCompile(`^(M?)(\d{1,2})/(M?)(\d{1,2})$`)
	tempOnlyRegex     = regexp.MustCompile(`^(M?)(\d{1,2})/(//)?$`)
	pressureRegex     = regexp.MustCompile(`^A(\d{4})$`)
	validRegex        = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
	cavokRegex        = regexp.MustCompile(`^CAVOK$`)
	// Enhanced runway condition regex that handles variable values, peak values and trend indicator
	// Updated to correctly capture trend indicator both with and without a preceding slash
	runwayCondRegex = regexp.MustCompile(`^R(\d{2}[CLR]?)/(([MP]?\d+)(V([MP]?\d+))?(FT)?)(/(U|D|N)|U|D|N)?$`)
	// Regex for cleared runway condition (e.g., R24C/CLRD62)
	runwayClearedRegex = regexp.MustCompile(`^R(\d{2}[CLR]?)/CLRD(\d{2}|//)$`)
	vvRegex            = regexp.MustCompile(`^VV(\d{3})$`)
	ndvRegex           = regexp.MustCompile(`^(\d{4,5})NDV$`)
	zeroWindRegex      = regexp.MustCompile(`^(VRB|\d{3})(0+)(KT|MPS)$`)
)

var (
	weatherGroupRegex     = regexp.MustCompile(`^(RE)?([-+]?(VC)?)(MI|PR|BC|DR|BL|SH|TS|FZ)?((DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS)*)$`)
	pressureMissingRegex  = regexp.MustCompile(`^[AQ]////$`)
	rvrMissingRegex       = regexp.MustCompile(`^R(\d{2}[CLR]?|//)/////$`)
	runwayStateRegex      = regexp.MustCompile(`^R(\d{2}[CLR]?)/([\d/])([\d/])([\d/]{2})([\d/]{2})$`)
	runwaySnowClosedRegex = regexp.MustCompile(`^(R(\d{2}[CLR]?)?/)?SNOCLO$`)
	seaStateRegex         = regexp.MustCompile(`^W(M?\d{2}|//)/(S([\d/])|H(\d{1,3}|/{1,3})|//?)$`)
	militaryColorRegex    = regexp.MustCompile(`^(BLACK)?(BLU|WHT|GRN|YLO[12]?|AMB|RED)\+?$`)
	tafFromRegex          = regexp.MustCompile(`^FM(\d{2})(\d{2})(\d{2})$`)
	tafDayTimeRegex       = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})$`)
	tafProbRegex          = regexp.MustCompile(`^PROB(\d{2})(TEMPO|INTER)?$`)
	trendTimeRegex        = regexp.MustCompile(`^(FM|TL|AT)(\d{2})(\d{2})$`)
	qfeRegex              = regexp.MustCompile(`^QFE ?(\d{3,4}(?:[./]\d)?)$`)
	gluedPressureRegex    = regexp.MustCompile(`^([AQ]\d{4})(NOSIG|TEMPO|BECMG|RMK)$`)
	tafTemperatureRegex   = regexp.MustCompile(`^T([XN]?)(M?)(\d{2})/(\d{2})(\d{2})Z*$`)
	tafQNHInchesRegex     = regexp.MustCompile(`^QNH(\d{4})INS$`)
	tafIcingRegex         = regexp.MustCompile(`^6(\d)(\d{3})(\d)$`)
	tafTurbulenceRegex    = regexp.MustCompile(`^5(\d)(\d{3})(\d)$`)
)

// WeatherData contains common fields for different weather reports
type WeatherData struct {
	Raw     string
	Station string
	Time    time.Time
}

// Wind represents wind information in a weather report
type Wind struct {
	Direction string
	Speed     *int
	Gust      int
	Unit      string
}

// WindShear represents wind shear information in a weather report
type WindShear struct {
	Type     string // "RWY" for runway or "ALT" for altitude
	Runway   string // Runway identifier (e.g., "12", "30L")
	Phase    string // "TKOF", "LDG", or "ALL"
	Altitude int    // Altitude in hundreds of feet (only for altitude type)
	Wind     Wind   // Wind information at the shear level (only for altitude type)
	Raw      string // Original raw string
}

// Cloud represents cloud information in a weather report
type Cloud struct {
	Coverage      string // "///" when not reported
	Height        int
	HeightMissing bool
	Type          string // CB, TCU, or "///" when not reported
}

// Remark represents a decoded remark from the RMK section
type Remark struct {
	Raw         string
	Description string
}

// SiteInfo represents the location information for a station
type SiteInfo struct {
	Name      string
	State     string
	Country   string
	Elevation *int // meters
}

// RunwayCondition represents a runway visual range group
type RunwayCondition struct {
	Runway     string // Runway identifier (e.g., "21", "24C", "27")
	Visibility int    // Visibility in feet or meters
	VisMin     int    // For variable visibility - minimum value
	VisMax     int    // For variable visibility - maximum value
	Trend      string // Trend indicator: "U" (upward), "D" (downward), or "N" (no change)
	Unit       string // "FT" for feet or "" for meters
	Prefix     string // Prefix if any: "P" (more than) or "M" (less than)
	Missing    bool   // RVR not reported (R24/////)
	Raw        string // Original raw string
}

// RunwayState represents a runway state group (deposits, depth and braking).
// Code fields hold the raw WMO digits, "/" or "//" when not reported.
type RunwayState struct {
	Runway     string // "88" means all runways, "99" repeats the last report
	Deposit    string
	Extent     string
	Depth      string
	Friction   string
	Cleared    bool // CLRD: contamination has ceased; Friction still applies
	SnowClosed bool
	Raw        string
}

// SeaState represents a sea surface temperature and sea state or wave height group
type SeaState struct {
	Temperature *int
	State       string // WMO code table 3700 digit, "" when not reported
	WaveHeight  *int   // Significant wave height in decimeters
	Raw         string
}

// PressureReading is a pressure group; a zero Value with a Unit means the group was reported missing
type PressureReading struct {
	Value float64
	Unit  string // "hPa" or "inHg"
}

// METAR represents a decoded METAR weather report
type METAR struct {
	WeatherData
	ReportType       string // "METAR" or "SPECI" when the report starts with one
	SiteInfo         SiteInfo
	Wind             Wind
	WindShear        []WindShear
	WindVariation    string // Wind direction variation (e.g., "360V040")
	Visibility       string
	MinVisibility    string // Directional minimum visibility (e.g., "1500NE")
	Weather          []string
	Clouds           []Cloud
	VertVis          *int // Vertical visibility in hundreds of feet; 0 means below 100 feet
	VertVisMissing   bool // VV///: sky obscured, height not measured
	Temperature      *int // Changed to pointer to represent missing value
	DewPoint         *int // Using pointer to represent missing dew point
	Pressure         float64
	PressureUnit     string          // "hPa" or "inHg"
	SecondPressure   PressureReading // a second QNH group, as in "Q1020 A3015"
	StationPressure  float64         // QFE in hPa, reported in the body by some South American stations
	Remarks          []Remark
	RunwayConditions []RunwayCondition // Runway visual range
	RunwayStates     []RunwayState     // Runway deposits, depth and braking
	SeaStates        []SeaState        // Sea surface temperature and sea state (W group)
	Trends           []Forecast        // TEMPO, BECMG, and INTER trend groups
	SpecialCodes     []string          // Special codes like AUTO, NOSIG, etc.
	Unhandled        []string
}

// HazardLayer is a military icing (6IhhhT) or turbulence (5Bhhht) forecast group
type HazardLayer struct {
	Intensity     string // WMO code table 1738 (turbulence) or 1734 (icing) digit
	BaseFeet      int
	ThicknessFeet int
	Raw           string
}

// TemperatureForecast is a TX (maximum), TN (minimum), or plain T forecast temperature group
type TemperatureForecast struct {
	Kind    string // "MAX", "MIN", or "" for a plain T group
	Celsius int
	At      time.Time
	Raw     string
}

// Forecast represents a single forecast period within a TAF
type Forecast struct {
	Type          string    // BASE, FM, TEMPO, BECMG, INTER, or PROB (a PROB without TEMPO/INTER)
	Probability   int       // PROBnn value; also set on a PROBnn TEMPO or PROBnn INTER period
	From          time.Time // Start time of this forecast period
	To            time.Time // End time of this forecast period (if applicable)
	At            time.Time // METAR trend AThhmm time
	Wind          Wind
	WindVariation string
	WindShear     []WindShear
	Visibility    string
	MinVisibility string // Directional minimum visibility (e.g., "1500NE")
	CAVOK         bool
	Weather       []string
	NSW           bool // no significant weather
	Clouds        []Cloud
	NSC           bool // no significant cloud
	VertVis       *int // Vertical visibility in hundreds of feet; 0 means below 100 feet
	Pressure      float64
	PressureUnit  string
	Icing         []HazardLayer
	Turbulence    []HazardLayer
	Unhandled     []string
	Raw           string // Raw text for this forecast period
	InEffect      bool   // in effect at the reference time passed to DecodeTAF
}

// TAF represents a decoded Terminal Aerodrome Forecast
type TAF struct {
	WeatherData
	SiteInfo     SiteInfo
	Amended      bool
	Corrected    bool
	Nil          bool
	Cancelled    bool
	ValidFrom    time.Time
	ValidTo      time.Time
	Forecasts    []Forecast
	Temperatures []TemperatureForecast
	Remarks      string // raw text after RMK
}
