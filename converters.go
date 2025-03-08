package main

import (
	"fmt"
	"time"
)

// CelsiusToFahrenheit converts temperature from Celsius to Fahrenheit
func CelsiusToFahrenheit(celsius int) int {
	return (celsius * 9 / 5) + 32
}

// InHgToMillibars converts pressure from inches of mercury to millibars (hPa)
func InHgToMillibars(inHg float64) float64 {
	return inHg * 33.8639
}

// Calculate the relative time string
func relativeTimeString(t time.Time) string {
	now := time.Now().UTC()
	diff := now.Sub(t)

	// Convert to minutes for easier comparisons
	minutes := int(diff.Minutes())

	if minutes < 0 {
		// For future times (rare, but possible with timezone issues)
		return "(in the future)"
	} else if minutes < 1 {
		return "(just now)"
	} else if minutes < 60 {
		return fmt.Sprintf("(%d minutes ago)", minutes)
	} else if minutes < 1440 { // less than 24 hours
		hours := minutes / 60
		mins := minutes % 60
		if mins == 0 {
			return fmt.Sprintf("(%d hours ago)", hours)
		}
		return fmt.Sprintf("(%d hours, %d minutes ago)", hours, mins)
	} else {
		days := minutes / 1440
		hours := (minutes % 1440) / 60
		if hours == 0 {
			return fmt.Sprintf("(%d days ago)", days)
		}
		return fmt.Sprintf("(%d days, %d hours ago)", days, hours)
	}
}
