package main

import (
	"regexp"
	"time"
)

// Common weather phenomena mapping used across the application
var weatherCodes = map[string]string{
	"VC": "vicinity",
	"+":  "heavy",
	"-":  "light",
	"MI": "shallow",
	"PR": "partial",
	"BC": "patches",
	"DR": "low drifting",
	"BL": "blowing",
	"SH": "shower",
	"TS": "thunderstorm",
	"FZ": "freezing",
	"DZ": "drizzle",
	"RA": "rain",
	"SN": "snow",
	"SG": "snow grains",
	"IC": "ice crystals",
	"PL": "ice pellets",
	"GR": "hail",
	"GS": "small hail",
	"UP": "unknown precipitation",
	"BR": "mist",
	"FG": "fog",
	"FU": "smoke",
	"VA": "volcanic ash",
	"DU": "widespread dust",
	"SA": "sand",
	"HZ": "haze",
	"PY": "spray",
	"PO": "dust whirls",
	"SQ": "squalls",
	"FC": "funnel cloud",
	"SS": "sandstorm",
	"DS": "duststorm",
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

// Common weather description mapping for simplified display
var weatherDescriptions = map[string]string{
	// Basic codes
	"BR":   "mist",
	"FG":   "fog",
	"-RA":  "light rain",
	"RA":   "rain",
	"+RA":  "heavy rain",
	"-SN":  "light snow",
	"SN":   "snow",
	"+SN":  "heavy snow",
	"VCSH": "showers in vicinity",
	"VCTS": "thunderstorm in vicinity",
	"TS":   "thunderstorm",
	"TSRA": "thunderstorm with rain",
	"DZ":   "drizzle",
	"-DZ":  "light drizzle",
	"+DZ":  "heavy drizzle",

	// Composite codes - showers
	"-SHRA": "light rain showers",
	"SHRA":  "rain showers",
	"+SHRA": "heavy rain showers",
	"-SHSN": "light snow showers",
	"SHSN":  "snow showers",
	"+SHSN": "heavy snow showers",
	"SHGR":  "hail showers",
	"-SHGR": "light hail showers",
	"+SHGR": "heavy hail showers",

	// Thunderstorms
	"+TS":   "heavy thunderstorm",
	"-TS":   "light thunderstorm",
	"-TSRA": "light thunderstorm with rain",
	"+TSRA": "heavy thunderstorm with rain",
	"TSSN":  "thunderstorm with snow",
	"-TSSN": "light thunderstorm with snow",
	"+TSSN": "heavy thunderstorm with snow",
	"TSGR":  "thunderstorm with hail",
	"+TSGR": "heavy thunderstorm with hail",

	// Freezing precipitation
	"FZRA":  "freezing rain",
	"-FZRA": "light freezing rain",
	"+FZRA": "heavy freezing rain",
	"FZDZ":  "freezing drizzle",
	"-FZDZ": "light freezing drizzle",
	"+FZDZ": "heavy freezing drizzle",

	// Blowing and drifting
	"BLSN": "blowing snow",
	"DRSN": "drifting snow",
	"BLDU": "blowing dust",
	"BLSA": "blowing sand",

	// Vicinity phenomena
	"VCFG": "fog in vicinity",
	"VCFC": "funnel cloud in vicinity",

	// Other combinations
	"MIFG": "shallow fog",
	"BCFG": "patches of fog",
	"PRFG": "partial fog",
	"FC":   "funnel cloud",
	"+FC":  "tornado/waterspout",
}

// Commonly used regular expressions
var (
	timeRegex     = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
	windRegex     = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	visRegexM     = regexp.MustCompile(`^(\d+(?:/\d+)?|M)SM$`)
	visRegexP     = regexp.MustCompile(`^(\d+(?:/\d+)?|M|P)(\d+)SM$`)
	cloudRegex    = regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})?(CB|TCU)?$`)
	tempRegex     = regexp.MustCompile(`^(M?)(\d{2})/(M?)(\d{2})$`)
	pressureRegex = regexp.MustCompile(`^A(\d{4})$`)
	validRegex    = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
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
	Speed     int
	Gust      int
	Unit      string
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

// METAR represents a decoded METAR weather report
type METAR struct {
	WeatherData
	Wind        Wind
	Visibility  string
	Weather     []string
	Clouds      []Cloud
	Temperature int
	DewPoint    int
	Pressure    float64
	Remarks     []Remark
}

// Forecast represents a single forecast period within a TAF
type Forecast struct {
	Type       string    // FM (from), TEMPO (temporary), BECMG (becoming), etc.
	From       time.Time // Start time of this forecast period
	To         time.Time // End time of this forecast period (if applicable)
	Wind       Wind
	Visibility string
	Weather    []string
	Clouds     []Cloud
	Raw        string // Raw text for this forecast period
}

// TAF represents a decoded Terminal Aerodrome Forecast
type TAF struct {
	WeatherData
	ValidFrom time.Time
	ValidTo   time.Time
	Forecasts []Forecast
}
