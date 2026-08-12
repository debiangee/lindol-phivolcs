package transform

import (
	"testing"
	"time"
)

func TestNormalize_ValidEntry(t *testing.T) {
	raw := RawEntry{
		DateText:    "6 August 2026 - 1:41 PM",
		LatText:     "5.21",
		LonText:     "125.23",
		DepthText:   "10",
		MagText:     "4.6",
		Location:    "040 km SW of Sarangani (Davao Occidental)",
		BulletinURL: "https://earthquake.phivolcs.dost.gov.ph/...",
	}

	clean, err := Normalize(raw)

	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if clean == nil {
		t.Fatal("Normalize() returned nil clean entry")
	}

	// Check basic fields
	if clean.Latitude != 5.21 {
		t.Errorf("Latitude = %f, want 5.21", clean.Latitude)
	}

	if clean.Longitude != 125.23 {
		t.Errorf("Longitude = %f, want 125.23", clean.Longitude)
	}

	if clean.DepthKm != 10.0 {
		t.Errorf("DepthKm = %f, want 10.0", clean.DepthKm)
	}

	if clean.Magnitude != 4.6 {
		t.Errorf("Magnitude = %f, want 4.6", clean.Magnitude)
	}

	if clean.Location == "" {
		t.Error("Location is empty")
	}
}

func TestNormalize_InvalidDate(t *testing.T) {
	raw := RawEntry{
		DateText:  "invalid date",
		LatText:   "5.21",
		LonText:   "125.23",
		DepthText: "10",
		MagText:   "4.6",
		Location:  "Test Location",
	}

	clean, err := Normalize(raw)

	if err == nil {
		t.Fatal("Normalize() expected error for invalid date, got nil")
	}

	if clean != nil {
		t.Error("Normalize() should return nil clean entry on error")
	}

	if err.Field != "date" {
		t.Errorf("Error field = %q, want %q", err.Field, "date")
	}
}

func TestNormalize_OutOfRangeLatitude(t *testing.T) {
	raw := RawEntry{
		DateText:  "6 August 2026 - 1:41 PM",
		LatText:   "50.0", // Outside PH range
		LonText:   "125.23",
		DepthText: "10",
		MagText:   "4.6",
		Location:  "Test Location",
	}

	clean, err := Normalize(raw)

	if err == nil {
		t.Fatal("Normalize() expected error for out-of-range latitude, got nil")
	}

	if clean != nil {
		t.Error("Normalize() should return nil clean entry on error")
	}

	if err.Field != "latitude" {
		t.Errorf("Error field = %q, want %q", err.Field, "latitude")
	}
}

func TestNormalize_InvalidMagnitude(t *testing.T) {
	raw := RawEntry{
		DateText:  "6 August 2026 - 1:41 PM",
		LatText:   "5.21",
		LonText:   "125.23",
		DepthText: "10",
		MagText:   "not a number",
		Location:  "Test Location",
	}

	clean, err := Normalize(raw)

	if err == nil {
		t.Fatal("Normalize() expected error for invalid magnitude, got nil")
	}

	if clean != nil {
		t.Error("Normalize() should return nil clean entry on error")
	}

	if err.Field != "magnitude" {
		t.Errorf("Error field = %q, want %q", err.Field, "magnitude")
	}
}

func TestNormalize_EmptyLocation(t *testing.T) {
	raw := RawEntry{
		DateText:  "6 August 2026 - 1:41 PM",
		LatText:   "5.21",
		LonText:   "125.23",
		DepthText: "10",
		MagText:   "4.6",
		Location:  "   ", // Only whitespace
	}

	clean, err := Normalize(raw)

	if err == nil {
		t.Fatal("Normalize() expected error for empty location, got nil")
	}

	if clean != nil {
		t.Error("Normalize() should return nil clean entry on error")
	}

	if err.Field != "location" {
		t.Errorf("Error field = %q, want %q", err.Field, "location")
	}
}

func TestNormalizeBatch(t *testing.T) {
	raws := []RawEntry{
		{
			DateText:  "6 August 2026 - 1:41 PM",
			LatText:   "5.21",
			LonText:   "125.23",
			DepthText: "10",
			MagText:   "4.6",
			Location:  "Test Location 1",
		},
		{
			DateText:  "invalid date", // This one should fail
			LatText:   "5.21",
			LonText:   "125.23",
			DepthText: "10",
			MagText:   "4.6",
			Location:  "Test Location 2",
		},
		{
			DateText:  "7 August 2026 - 2:30 PM",
			LatText:   "6.50",
			LonText:   "126.00",
			DepthText: "15",
			MagText:   "3.2",
			Location:  "Test Location 3",
		},
	}

	result := NormalizeBatch(raws)

	if len(result.Entries) != 2 {
		t.Errorf("NormalizeBatch() got %d valid entries, want 2", len(result.Entries))
	}

	if len(result.Errors) != 1 {
		t.Errorf("NormalizeBatch() got %d errors, want 1", len(result.Errors))
	}

	if len(result.Errors) > 0 && result.Errors[0].Field != "date" {
		t.Errorf("First error field = %q, want %q", result.Errors[0].Field, "date")
	}
}

func TestNormalize_DateFormats(t *testing.T) {
	formats := []string{
		"6 August 2026 - 1:41 PM",
		"06 August 2026 - 1:41 PM",
		"6 August 2026 - 01:41 PM",
		"06 August 2026 - 01:41 PM",
	}

	for _, dateStr := range formats {
		t.Run(dateStr, func(t *testing.T) {
			raw := RawEntry{
				DateText:  dateStr,
				LatText:   "5.21",
				LonText:   "125.23",
				DepthText: "10",
				MagText:   "4.6",
				Location:  "Test Location",
			}

			clean, err := Normalize(raw)

			if err != nil {
				t.Errorf("Normalize() with date format %q failed: %v", dateStr, err)
			}

			if clean == nil {
				t.Errorf("Normalize() with date format %q returned nil", dateStr)
				return
			}

			// Check that time is in Philippine timezone (UTC+8)
			loc := time.FixedZone("PHT", 8*60*60)
			if clean.DateTime.Location().String() != loc.String() {
				t.Errorf("DateTime location = %v, want PHT", clean.DateTime.Location())
			}
		})
	}
}

