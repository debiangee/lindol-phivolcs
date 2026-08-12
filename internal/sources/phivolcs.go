// Package sources handles data fetching from external earthquake sources.
package sources

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/debiangee/lindol-api/internal/transform"
)

const phivolcsBaseURL = "https://earthquake.phivolcs.dost.gov.ph/"

// PhivolcsEntry represents a single earthquake entry scraped from PHIVOLCS.
type PhivolcsEntry struct {
	DateTime    time.Time
	Latitude    float64
	Longitude   float64
	DepthKm     float64
	Magnitude   float64
	Location    string
	BulletinURL string
}

// PhivolcsBulletin holds enrichment data from a PHIVOLCS bulletin page.
type PhivolcsBulletin struct {
	Intensity string
	FeltAreas []string
}

// PhivolcsScrapeResult holds the outcome of a scrape with transform errors.
type PhivolcsScrapeResult struct {
	Entries         []PhivolcsEntry
	TransformErrors []transform.TransformError
}

// PhivolcsClient scrapes earthquake data from the PHIVOLCS website.
type PhivolcsClient struct {
	client *http.Client
	logger *slog.Logger
}

// NewPhivolcsClient creates a new PHIVOLCS scraper.
func NewPhivolcsClient(logger *slog.Logger) *PhivolcsClient {
	return &PhivolcsClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// FetchRecentEntries scrapes the PHIVOLCS main listing page, normalizes via transformer,
// and returns parsed entries along with any transform errors.
func (p *PhivolcsClient) FetchRecentEntries(ctx context.Context) (*PhivolcsScrapeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, phivolcsBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "lindol-api/1.0 (earthquake monitoring)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch PHIVOLCS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PHIVOLCS returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	// Extract raw entries from HTML
	var rawEntries []transform.RawEntry

	doc.Find("table.MsoNormalTable tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 6 {
			return
		}

		raw := p.extractRawRow(cells)
		if raw != nil {
			rawEntries = append(rawEntries, *raw)
		}
	})

	// Run through transformer
	result := transform.NormalizeBatch(rawEntries)

	// Convert clean entries to PhivolcsEntry
	var entries []PhivolcsEntry
	for _, clean := range result.Entries {
		entries = append(entries, PhivolcsEntry{
			DateTime:    clean.DateTime,
			Latitude:    clean.Latitude,
			Longitude:   clean.Longitude,
			DepthKm:     clean.DepthKm,
			Magnitude:   clean.Magnitude,
			Location:    clean.Location,
			BulletinURL: clean.BulletinURL,
		})
	}

	p.logger.Debug("PHIVOLCS fetch complete",
		"entries", len(entries),
		"transform_errors", len(result.Errors),
		"raw_rows", len(rawEntries),
	)

	return &PhivolcsScrapeResult{
		Entries:         entries,
		TransformErrors: result.Errors,
	}, nil
}

// extractRawRow extracts raw text values from a table row (no parsing/validation).
func (p *PhivolcsClient) extractRawRow(cells *goquery.Selection) *transform.RawEntry {
	dateCell := cells.Eq(0)
	dateText := strings.TrimSpace(dateCell.Text())
	if dateText == "" {
		return nil
	}

	// Get bulletin link
	bulletinURL := ""
	link := dateCell.Find("a")
	if href, exists := link.Attr("href"); exists {
		bulletinURL = href
	}

	return &transform.RawEntry{
		DateText:    dateText,
		LatText:     strings.TrimSpace(cells.Eq(1).Text()),
		LonText:     strings.TrimSpace(cells.Eq(2).Text()),
		DepthText:   strings.TrimSpace(cells.Eq(3).Text()),
		MagText:     strings.TrimSpace(cells.Eq(4).Text()),
		Location:    strings.TrimSpace(cells.Eq(5).Text()),
		BulletinURL: bulletinURL,
	}
}

// FetchBulletin scrapes a specific PHIVOLCS bulletin page for intensity details.
func (p *PhivolcsClient) FetchBulletin(ctx context.Context, bulletinURL string) (*PhivolcsBulletin, error) {
	// Make URL absolute if relative
	if !strings.HasPrefix(bulletinURL, "http") {
		bulletinURL = phivolcsBaseURL + bulletinURL
	}
	// Normalize backslashes to forward slashes
	bulletinURL = strings.ReplaceAll(bulletinURL, "\\", "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bulletinURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "lindol-api/1.0 (earthquake monitoring)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bulletin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bulletin returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse bulletin HTML: %w", err)
	}

	bulletin := &PhivolcsBulletin{}

	bodyText := doc.Find("body").Text()

	// Look for reported intensities
	intensityPattern := regexp.MustCompile(`Intensity\s+(I{1,3}V?I{0,3}|[IVX]+)\s*[-\x{2013}\x{2014}]`)
	matches := intensityPattern.FindAllStringSubmatch(bodyText, -1)
	if len(matches) > 0 {
		bulletin.Intensity = findMaxIntensity(matches)
	}

	// Extract felt areas
	areaPattern := regexp.MustCompile(`Intensity\s+[IVX]+\s*[-\x{2013}\x{2014}]\s*(.+?)(?:\n|Intensity|$)`)
	areaMatches := areaPattern.FindAllStringSubmatch(bodyText, -1)
	for _, match := range areaMatches {
		if len(match) > 1 {
			areas := parseAreas(match[1])
			bulletin.FeltAreas = append(bulletin.FeltAreas, areas...)
		}
	}

	return bulletin, nil
}

// MatchEntry finds the best matching PHIVOLCS entry for a given earthquake.
func MatchEntry(entries []PhivolcsEntry, eventTime time.Time, lat, lon float64, timeWindowMin int, distDeg float64) *PhivolcsEntry {
	var bestMatch *PhivolcsEntry
	bestScore := math.MaxFloat64

	for i := range entries {
		entry := &entries[i]

		// Time difference in minutes
		timeDiff := math.Abs(entry.DateTime.Sub(eventTime).Minutes())
		if timeDiff > float64(timeWindowMin) {
			continue
		}

		// Location difference in degrees
		latDiff := math.Abs(entry.Latitude - lat)
		lonDiff := math.Abs(entry.Longitude - lon)
		dist := math.Sqrt(latDiff*latDiff + lonDiff*lonDiff)
		if dist > distDeg {
			continue
		}

		// Combined score (lower is better)
		score := timeDiff + dist*10
		if score < bestScore {
			bestScore = score
			bestMatch = entry
		}
	}

	return bestMatch
}

// GeneratePhivolcsID creates a unique ID for a PHIVOLCS entry (they don't have one).
// Based on hash of datetime + lat + lon + magnitude.
func GeneratePhivolcsID(entry PhivolcsEntry) string {
	return fmt.Sprintf("phivolcs_%s_%.2f_%.2f_%.1f",
		entry.DateTime.UTC().Format("20060102T150405"),
		entry.Latitude,
		entry.Longitude,
		entry.Magnitude,
	)
}

// --- Helpers ---

func parseAreas(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	parts := regexp.MustCompile(`[;,]|\band\b`).Split(s, -1)
	var areas []string
	for _, part := range parts {
		area := strings.TrimSpace(part)
		if area != "" && len(area) > 2 {
			areas = append(areas, area)
		}
	}
	return areas
}

func findMaxIntensity(matches [][]string) string {
	maxVal := 0
	maxStr := ""

	for _, match := range matches {
		if len(match) > 1 {
			val := romanToInt(match[1])
			if val > maxVal {
				maxVal = val
				maxStr = match[1]
			}
		}
	}
	return maxStr
}

func romanToInt(s string) int {
	romanMap := map[byte]int{
		'I': 1, 'V': 5, 'X': 10,
	}

	total := 0
	for i := 0; i < len(s); i++ {
		val := romanMap[s[i]]
		if i+1 < len(s) && romanMap[s[i+1]] > val {
			total -= val
		} else {
			total += val
		}
	}
	return total
}
