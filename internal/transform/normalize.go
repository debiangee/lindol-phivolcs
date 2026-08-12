// Package transform handles data normalization and validation for scraped entries.
package transform

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// RawEntry represents a raw scraped entry before normalization.
type RawEntry struct {
	DateText    string
	LatText     string
	LonText     string
	DepthText   string
	MagText     string
	Location    string
	BulletinURL string
}

// CleanEntry represents a fully validated and normalized earthquake entry.
type CleanEntry struct {
	DateTime    time.Time
	Latitude    float64
	Longitude   float64
	DepthKm     float64
	Magnitude   float64
	Location    string
	BulletinURL string
}

// TransformError contains details about a failed transformation.
type TransformError struct {
	RawEntry RawEntry
	Field    string
	RawValue string
	Err      error
}

func (e *TransformError) Error() string {
	return fmt.Sprintf("transform failed on field %q (value: %q): %v", e.Field, e.RawValue, e.Err)
}

// TransformResult holds the outcome of a batch transformation.
type TransformResult struct {
	Entries []CleanEntry
	Errors  []TransformError
}

// Normalize converts a raw scraped entry into a clean, validated entry.
// Returns the clean entry or a TransformError with details about what failed.
func Normalize(raw RawEntry) (*CleanEntry, *TransformError) {
	clean := &CleanEntry{}

	// Parse date
	dt, err := parseDate(raw.DateText)
	if err != nil {
		return nil, &TransformError{RawEntry: raw, Field: "date", RawValue: raw.DateText, Err: err}
	}
	clean.DateTime = dt

	// Parse latitude
	lat, err := parseCoordinate(raw.LatText)
	if err != nil {
		return nil, &TransformError{RawEntry: raw, Field: "latitude", RawValue: raw.LatText, Err: err}
	}
	if lat < 1.0 || lat > 22.0 {
		return nil, &TransformError{RawEntry: raw, Field: "latitude", RawValue: raw.LatText, Err: fmt.Errorf("out of PH range (1-22): %.4f", lat)}
	}
	clean.Latitude = lat

	// Parse longitude
	lon, err := parseCoordinate(raw.LonText)
	if err != nil {
		return nil, &TransformError{RawEntry: raw, Field: "longitude", RawValue: raw.LonText, Err: err}
	}
	if lon < 115.0 || lon > 130.0 {
		return nil, &TransformError{RawEntry: raw, Field: "longitude", RawValue: raw.LonText, Err: fmt.Errorf("out of PH range (115-130): %.4f", lon)}
	}
	clean.Longitude = lon

	// Parse depth
	depth, err := parseDepth(raw.DepthText)
	if err != nil {
		return nil, &TransformError{RawEntry: raw, Field: "depth", RawValue: raw.DepthText, Err: err}
	}
	if depth < 0 || depth > 700 {
		return nil, &TransformError{RawEntry: raw, Field: "depth", RawValue: raw.DepthText, Err: fmt.Errorf("out of range (0-700): %.1f", depth)}
	}
	clean.DepthKm = depth

	// Parse magnitude
	mag, err := parseMagnitude(raw.MagText)
	if err != nil {
		return nil, &TransformError{RawEntry: raw, Field: "magnitude", RawValue: raw.MagText, Err: err}
	}
	if mag < 0 || mag > 10 {
		return nil, &TransformError{RawEntry: raw, Field: "magnitude", RawValue: raw.MagText, Err: fmt.Errorf("out of range (0-10): %.1f", mag)}
	}
	clean.Magnitude = mag

	// Normalize location text
	clean.Location = normalizeLocation(raw.Location)
	if clean.Location == "" {
		return nil, &TransformError{RawEntry: raw, Field: "location", RawValue: raw.Location, Err: fmt.Errorf("empty after normalization")}
	}

	// Normalize bulletin URL
	clean.BulletinURL = normalizeBulletinURL(raw.BulletinURL)

	return clean, nil
}

// NormalizeBatch processes multiple raw entries and returns results with errors.
func NormalizeBatch(raws []RawEntry) TransformResult {
	result := TransformResult{}

	for _, raw := range raws {
		clean, err := Normalize(raw)
		if err != nil {
			result.Errors = append(result.Errors, *err)
			continue
		}
		result.Entries = append(result.Entries, *clean)
	}

	return result
}

// --- Parsers ---

func parseDate(s string) (time.Time, error) {
	s = cleanText(s)

	// Load Philippine timezone (UTC+8)
	loc := time.FixedZone("PHT", 8*60*60)

	formats := []string{
		"2 January 2006 - 3:04 PM",
		"02 January 2006 - 3:04 PM",
		"2 January 2006 - 03:04 PM",
		"02 January 2006 - 03:04 PM",
		"2 January 2006 - 3:04PM",
		"02 January 2006 - 3:04PM",
		"2 January 2006 - 15:04",
		"02 January 2006 - 15:04",
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("no format matched: %q", s)
}

func parseCoordinate(s string) (float64, error) {
	s = cleanText(s)
	// Remove degree symbols and direction letters
	s = strings.TrimRight(s, "°NnSsEeWw ")
	s = strings.ReplaceAll(s, "Â°", "")
	s = strings.ReplaceAll(s, "°", "")

	if s == "" {
		return 0, fmt.Errorf("empty coordinate")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", s)
	}

	return f, nil
}

func parseDepth(s string) (float64, error) {
	s = cleanText(s)
	// Remove "km" suffix if present
	s = strings.TrimSuffix(strings.ToLower(s), "km")
	s = strings.TrimSpace(s)

	if s == "" {
		return 0, fmt.Errorf("empty depth")
	}

	// Handle zero-padded values like "003"
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", s)
	}

	return f, nil
}

func parseMagnitude(s string) (float64, error) {
	s = cleanText(s)

	if s == "" {
		return 0, fmt.Errorf("empty magnitude")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", s)
	}

	return f, nil
}

// --- Normalizers ---

func normalizeLocation(s string) string {
	s = fixEncoding(s)
	s = collapseWhitespace(s)
	s = strings.TrimSpace(s)
	return s
}

func normalizeBulletinURL(s string) string {
	s = strings.TrimSpace(s)
	// Convert backslashes to forward slashes
	s = strings.ReplaceAll(s, "\\", "/")
	return s
}

// --- Helpers ---

// cleanText removes extra whitespace, newlines, and non-printable characters.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = collapseWhitespace(s)
	// Remove non-printable characters except spaces
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// fixEncoding repairs common encoding issues from PHIVOLCS HTML.
func fixEncoding(s string) string {
	replacements := []struct{ old, new string }{
		{"Â°", "\u00B0"},
		{"Ã±", "\u00F1"},
		{"\xe2\x80\x93", "\u2013"},
		{"\xe2\x80\x94", "\u2014"},
		{"&amp;", "&"},
		{"&nbsp;", " "},
		{"&#39;", "'"},
		{"&quot;", "\""},
	}

	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}

	return s
}

// collapseWhitespace replaces multiple consecutive whitespace characters with a single space.
func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}
