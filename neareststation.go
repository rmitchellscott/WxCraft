package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
)

// Position represents a geographic coordinate
type Position struct {
	Latitude  float64
	Longitude float64
}

// Station represents an airport or weather station from the AWC API
type Station struct {
	ICAO      string  `json:"icaoId"`
	Name      string  `json:"name"`
	State     string  `json:"state"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Elevation int     `json:"elev"`
}

// Regular expression for matching US zipcodes
var zipRegex = regexp.MustCompile(`^\d{5}(-\d{4})?$`)

// degreesToRadians converts degrees to radians
func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// calculateDistance uses the Haversine formula to determine the distance between two points
func calculateDistance(pos1, pos2 Position) float64 {
	// Earth's radius in miles
	earthRadius := 3958.8 // miles

	// Convert latitude and longitude from degrees to radians
	lat1 := degreesToRadians(pos1.Latitude)
	lon1 := degreesToRadians(pos1.Longitude)
	lat2 := degreesToRadians(pos2.Latitude)
	lon2 := degreesToRadians(pos2.Longitude)

	// Haversine formula
	dLat := lat2 - lat1
	dLon := lon2 - lon1
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}

// createBoundingBox creates a bounding box around a position with the given radius in miles
func createBoundingBox(pos Position, radiusMiles float64) (minLat, minLon, maxLat, maxLon float64) {
	// Approximate degrees latitude per mile (roughly 1 degree = 69 miles)
	degreesPerMile := 1.0 / 69.0

	// Longitude degrees per mile varies with latitude
	latRad := degreesToRadians(pos.Latitude)
	lonDegreesPerMile := degreesPerMile / math.Cos(latRad)

	// Calculate the box
	minLat = pos.Latitude - (radiusMiles * degreesPerMile)
	maxLat = pos.Latitude + (radiusMiles * degreesPerMile)
	minLon = pos.Longitude - (radiusMiles * lonDegreesPerMile)
	maxLon = pos.Longitude + (radiusMiles * lonDegreesPerMile)

	return minLat, minLon, maxLat, maxLon
}

// findNearbyStations queries the Aviation Weather Center API to find stations near a position
func findNearbyStations(position Position, radiusMiles float64) ([]Station, error) {
	// Create bounding box
	minLat, minLon, maxLat, maxLon := createBoundingBox(position, radiusMiles)

	// Construct bounding box parameter
	bbox := fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", minLat, minLon, maxLat, maxLon)

	// Build API URL
	baseURL := "https://aviationweather.gov/api/data/stationinfo"
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("bbox", bbox)
	q.Set("format", "json") // Ensure we get JSON response
	u.RawQuery = q.Encode()

	// Make the request
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query Aviation Weather API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Aviation Weather API returned non-OK status: %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %w", err)
	}

	var stations []Station
	if err := json.Unmarshal(body, &stations); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	return stations, nil
}

// GetNearestAirportICAO finds the nearest airport's ICAO code
func GetNearestAirportICAO(latitude, longitude float64, searchRadiusMiles float64) (string, float64, error) {
	position := Position{
		Latitude:  latitude,
		Longitude: longitude,
	}

	// Find nearby airports
	stations, err := findNearbyStations(position, searchRadiusMiles)
	if err != nil {
		return "", 0, err
	}

	if len(stations) == 0 {
		return "", 0, fmt.Errorf("no airports found within %.1f miles", searchRadiusMiles)
	}

	// Calculate distances and sort
	type stationDistance struct {
		station  Station
		distance float64
	}

	var stationsWithDistance []stationDistance
	for _, station := range stations {
		stationPos := Position{
			Latitude:  station.Latitude,
			Longitude: station.Longitude,
		}
		distance := calculateDistance(position, stationPos)
		stationsWithDistance = append(stationsWithDistance, stationDistance{
			station:  station,
			distance: distance,
		})
	}

	// Sort by distance
	sort.Slice(stationsWithDistance, func(i, j int) bool {
		return stationsWithDistance[i].distance < stationsWithDistance[j].distance
	})

	if len(stationsWithDistance) == 0 {
		return "", 0, fmt.Errorf("failed to find nearest airport")
	}

	return stationsWithDistance[0].station.ICAO, stationsWithDistance[0].distance, nil
}

// ProcessAutoCommand handles the AUTO command to find the nearest airport
func ProcessAutoCommand(radiusMiles float64) (string, error) {
	fmt.Println("Finding nearest airport to your location...")
	location, err := GetLocation()
	if err != nil {
		return "", fmt.Errorf("failed to get your location: %v", err)
	}

	fmt.Printf("Your location: %s, %s (%.4f, %.4f)\n",
		location.City, location.Country,
		location.Latitude, location.Longitude)

	// Get the nearest airport ICAO code
	fmt.Printf("Searching for airports within %.1f miles...\n", radiusMiles)
	icaoCode, distance, err := GetNearestAirportICAO(
		location.Latitude,
		location.Longitude,
		radiusMiles,
	)

	if err != nil {
		return "", err
	}

	fmt.Printf("Nearest airport: %s (%.1f miles away)\n", icaoCode, distance)
	return icaoCode, nil
}

// ProcessZipcode handles the zipcode input to find the nearest airport
func ProcessZipcode(zipcode string, radiusMiles float64) (string, error) {
	fmt.Printf("Looking up location for zipcode %s...\n", zipcode)
	location, err := GetLocationByZipcode(zipcode)
	if err != nil {
		return "", fmt.Errorf("failed to get location for zipcode: %v", err)
	}

	fmt.Printf("Zipcode location: %s, %s, %s (%.4f, %.4f)\n",
		location.City, location.Region, location.Country,
		location.Latitude, location.Longitude)

	// Get the nearest airport ICAO code
	fmt.Printf("Searching for airports within %.1f miles...\n", radiusMiles)
	icaoCode, distance, err := GetNearestAirportICAO(
		location.Latitude,
		location.Longitude,
		radiusMiles,
	)

	if err != nil {
		return "", err
	}

	fmt.Printf("Nearest airport: %s (%.1f miles away)\n", icaoCode, distance)
	return icaoCode, nil
}
