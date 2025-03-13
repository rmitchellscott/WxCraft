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
	"CLR": "slear",
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
}

// TAF forecast types
var forecastTypes = map[string]string{
	"FM":     "from",
	"BECMG":  "becoming",
	"TEMPO":  "temporary",
	"PROB30": "30% probability of",
	"PROB40": "40% probability of",
	"INTER":  "intermittent",
}

// Commonly used regular expressions
var (
	timeRegex         = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
	windRegex         = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$|^(0+)(G\d{2})?KT$`)
	windRegexMPS      = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?MPS$|^(0+)(G\d{2})?MPS$`)
	windVarRegex      = regexp.MustCompile(`^(\d{3})V(\d{3})$`)
	windShearAltRegex = regexp.MustCompile(`^WS(\d{3})/(\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	windShearRwyRegex = regexp.MustCompile(`^WS(\s+(TKOF|LDG|ALL)\s+RWY(\d{2}[LCR]?)?|\s+R(\d{2}[LCR]?)?)$`)
	visRegexM         = regexp.MustCompile(`^M?(\d+(?:/\d+)?)SM$`)
	visRegexP         = regexp.MustCompile(`^(\d+(?:/\d+)?|M|P)(\d+)SM$`)
	visRegexNum       = regexp.MustCompile(`^\d{4}$`)
	visRegexDir       = regexp.MustCompile(`^(\d{4})([NESW]{1,2})$`)
	cloudRegex        = regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})?(CB|TCU)?$`)
	tempRegex         = regexp.MustCompile(`^(M?)(\d{2})/(M?)(\d{2})$`)
	tempOnlyRegex     = regexp.MustCompile(`^(M?)(\d{2})/$`)
	pressureRegex     = regexp.MustCompile(`^A(\d{4})$`)
	validRegex        = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
	probRegex         = regexp.MustCompile(`^PROB(\d{2})$`)
	cavokRegex        = regexp.MustCompile(`^CAVOK$`)
	rvrRegex          = regexp.MustCompile(`^R(\d{2}[CLR]?)/([MP]?\d+)([DNU])?$`)
	// Enhanced runway condition regex that handles variable values, peak values and trend indicator
	// Updated to correctly capture trend indicator both with and without a preceding slash
	runwayCondRegex = regexp.MustCompile(`^R(\d{2}[CLR]?)/(([MP]?\d+)(V([MP]?\d+))?(FT)?)(/(U|D|N)|U|D|N)?$`)
	// Regex for cleared runway condition (e.g., R24C/CLRD62)
	runwayClearedRegex = regexp.MustCompile(`^R(\d{2}[CLR]?)/CLRD(\d{2})$`)
	vvRegex            = regexp.MustCompile(`^VV(\d{3})$`)
	ndvRegex           = regexp.MustCompile(`^(\d{4,5})NDV$`)
	eWindRegex         = regexp.MustCompile(`^E(\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	extCloudRegex      = regexp.MustCompile(`^(FEW|SCT|BKN|OVC)(CB|TCU)(\d{3})$`)
	specialRegex       = regexp.MustCompile(`^(NOSIG|AUTO|COR|CCA|NSC|NCD|RTD)$`)
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
	Coverage string
	Height   int
	Type     string // CB, TCU, etc.
}

// Remark represents a decoded remark from the RMK section
type Remark struct {
	Raw         string
	Description string
}

// SiteInfo represents the location information for a station
type SiteInfo struct {
	Name    string
	State   string
	Country string
}

// RunwayCondition represents runway visual range and surface conditions information
type RunwayCondition struct {
	Runway      string // Runway identifier (e.g., "21", "24C", "27")
	Visibility  int    // Visibility in feet or meters
	VisMin      int    // For variable visibility - minimum value
	VisMax      int    // For variable visibility - maximum value
	Trend       string // Trend indicator: "U" (upward), "D" (downward), or "N" (no change)
	Unit        string // "FT" for feet or "" for meters
	Prefix      string // Prefix if any: "P" (more than) or "M" (less than)
	Cleared     bool   // Whether the runway is cleared
	ClearedTime int    // Time when runway was cleared (in minutes) for CLRD format
	Raw         string // Original raw string
}

// METAR represents a decoded METAR weather report
type METAR struct {
	WeatherData
	SiteInfo         SiteInfo
	Wind             Wind
	WindShear        []WindShear
	WindVariation    string // Wind direction variation (e.g., "360V040")
	Visibility       string
	Weather          []string
	Clouds           []Cloud
	VertVis          int  // Vertical visibility in hundreds of feet
	Temperature      *int // Changed to pointer to represent missing value
	DewPoint         *int // Using pointer to represent missing dew point
	Pressure         float64
	PressureUnit     string // "hPa" or "inHg"
	Remarks          []Remark
	RunwayConditions []RunwayCondition // Detailed runway visual range and conditions
	RVR              []string          // Legacy RVR field (maintained for compatibility)
	SpecialCodes     []string          // Special codes like AUTO, NOSIG, etc.
	Unhandled        []string
}

// Forecast represents a single forecast period within a TAF
type Forecast struct {
	Type        string    // FM (from), TEMPO (temporary), BECMG (becoming), PROB30, PROB40, etc.
	Probability int       // For PROB forecasts, the probability value (30, 40, etc.)
	From        time.Time // Start time of this forecast period
	To          time.Time // End time of this forecast period (if applicable)
	Wind        Wind
	WindShear   []WindShear
	Visibility  string
	Weather     []string
	Clouds      []Cloud
	VertVis     int    // Vertical visibility in hundreds of feet
	Raw         string // Raw text for this forecast period
}

// TAF represents a decoded Terminal Aerodrome Forecast
type TAF struct {
	WeatherData
	SiteInfo  SiteInfo
	ValidFrom time.Time
	ValidTo   time.Time
	Forecasts []Forecast
}
