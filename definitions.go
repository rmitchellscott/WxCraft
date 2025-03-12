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

// Special aerodrome conditions
var specialConditions = map[string]string{
	"NOSIG": "no significant changes expected",
	"AUTO":  "automated observation",
	"COR":   "corrected report",
	"CCA":   "corrected report",
	"NSC":   "no significant clouds",
	"NCD":   "no clouds detected",
	"CAVOK": "ceiling and visibility OK",
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
	timeRegex     = regexp.MustCompile(`^(\d{2})(\d{2})(\d{2})Z$`)
	windRegex     = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	windRegexMPS  = regexp.MustCompile(`^(VRB|\d{3})(\d{2,3})(G(\d{2,3}))?MPS$`)
	windVarRegex  = regexp.MustCompile(`^(\d{3})V(\d{3})$`)
	visRegexM     = regexp.MustCompile(`^M?(\d+(?:/\d+)?)SM$`)
	visRegexP     = regexp.MustCompile(`^(\d+(?:/\d+)?|M|P)(\d+)SM$`)
	visRegexNum   = regexp.MustCompile(`^\d{4}$`)
	visRegexDir   = regexp.MustCompile(`^(\d{4})([NESW]{1,2})$`)
	cloudRegex    = regexp.MustCompile(`^(SKC|CLR|FEW|SCT|BKN|OVC)(\d{3})?(CB|TCU)?$`)
	tempRegex     = regexp.MustCompile(`^(M?)(\d{2})/(M?)(\d{2})$`)
	tempOnlyRegex = regexp.MustCompile(`^(M?)(\d{2})/$`)
	pressureRegex = regexp.MustCompile(`^A(\d{4})$`)
	validRegex    = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)
	probRegex     = regexp.MustCompile(`^PROB(\d{2})$`)
	cavokRegex    = regexp.MustCompile(`^CAVOK$`)
	rvrRegex      = regexp.MustCompile(`^R(\d{2}[CLR]?)/([MP]?\d+)([DNU])?$`)
	vvRegex       = regexp.MustCompile(`^VV(\d{3})$`)
	ndvRegex      = regexp.MustCompile(`^(\d{4,5})NDV$`)
	eWindRegex    = regexp.MustCompile(`^E(\d{3})(\d{2,3})(G(\d{2,3}))?KT$`)
	extCloudRegex = regexp.MustCompile(`^(FEW|SCT|BKN|OVC)(CB|TCU)(\d{3})$`)
	specialRegex  = regexp.MustCompile(`^(NOSIG|AUTO|COR|CCA|NSC|NCD)$`)
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

// SiteInfo represents the location information for a station
type SiteInfo struct {
	Name    string
	State   string
	Country string
}

// METAR represents a decoded METAR weather report
type METAR struct {
	WeatherData
	SiteInfo      SiteInfo
	Wind          Wind
	WindVariation string // Wind direction variation (e.g., "360V040")
	Visibility    string
	Weather       []string
	Clouds        []Cloud
	VertVis       int     // Vertical visibility in hundreds of feet
	Temperature   int
	DewPoint      *int    // Using pointer to represent missing dew point
	Pressure      float64
	PressureUnit  string // "hPa" or "inHg"
	Remarks       []Remark
	RVR           []string // Runway Visual Range
	SpecialCodes  []string // Special codes like AUTO, NOSIG, etc.
}

// Forecast represents a single forecast period within a TAF
type Forecast struct {
	Type        string    // FM (from), TEMPO (temporary), BECMG (becoming), PROB30, PROB40, etc.
	Probability int       // For PROB forecasts, the probability value (30, 40, etc.)
	From        time.Time // Start time of this forecast period
	To          time.Time // End time of this forecast period (if applicable)
	Wind        Wind
	Visibility  string
	Weather     []string
	Clouds      []Cloud
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
