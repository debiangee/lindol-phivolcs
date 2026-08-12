// Package models defines data structures used across the application.
package models

import "time"

// Earthquake represents a seismic event stored in the database.
type Earthquake struct {
	ID                  string    `json:"id"`
	Magnitude           float64   `json:"magnitude"`
	Latitude            float64   `json:"latitude"`
	Longitude           float64   `json:"longitude"`
	DepthKm             float64   `json:"depth_km"`
	EventTime           time.Time `json:"event_time"`
	LocationDescription string    `json:"location_description"`

	// PHIVOLCS enrichment (nullable)
	PhivolcsIntensity  *string  `json:"phivolcs_intensity,omitempty"`
	PhivolcsFeltAreas  []string `json:"phivolcs_felt_areas,omitempty"`
	PhivolcsBulletinURL *string `json:"phivolcs_bulletin_url,omitempty"`
	Enriched           bool     `json:"enriched"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// USGSFeatureCollection is the top-level GeoJSON response from USGS.
type USGSFeatureCollection struct {
	Type     string        `json:"type"`
	Metadata USGSMetadata  `json:"metadata"`
	Features []USGSFeature `json:"features"`
}

// USGSMetadata contains info about the USGS response.
type USGSMetadata struct {
	Generated int64  `json:"generated"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Count     int    `json:"count"`
}

// USGSFeature is a single earthquake event from USGS GeoJSON.
type USGSFeature struct {
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Properties USGSProperties     `json:"properties"`
	Geometry   USGSGeometry       `json:"geometry"`
}

// USGSProperties holds the metadata for a USGS earthquake event.
type USGSProperties struct {
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	Time    int64   `json:"time"`    // Unix timestamp in milliseconds
	Updated int64   `json:"updated"` // Unix timestamp in milliseconds
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Type    string  `json:"type"`
}

// USGSGeometry holds the coordinates [longitude, latitude, depth].
type USGSGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [lon, lat, depth_km]
}

// ToEarthquake converts a USGS feature to our internal Earthquake model.
func (f *USGSFeature) ToEarthquake() Earthquake {
	now := time.Now().UTC()
	eventTime := time.UnixMilli(f.Properties.Time).UTC()

	var lat, lon, depth float64
	if len(f.Geometry.Coordinates) >= 3 {
		lon = f.Geometry.Coordinates[0]
		lat = f.Geometry.Coordinates[1]
		depth = f.Geometry.Coordinates[2]
	}

	return Earthquake{
		ID:                  f.ID,
		Magnitude:           f.Properties.Mag,
		Latitude:            lat,
		Longitude:           lon,
		DepthKm:             depth,
		EventTime:           eventTime,
		LocationDescription: f.Properties.Place,
		Enriched:            false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}
