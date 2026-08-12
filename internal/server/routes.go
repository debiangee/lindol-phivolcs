package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/debiangee/lindol-api/internal/services"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleRoot)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/earthquakes", s.handleListEarthquakes)
	s.mux.HandleFunc("GET /api/earthquakes/latest", s.handleLatestEarthquake)
	s.mux.HandleFunc("GET /api/earthquakes/{id}", s.handleGetEarthquake)
}

// --- Root ---

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "lindol-api",
		"status":  "testing",
		"message": "This is a testing deployment and is not intended for production use.",
	})
}

// --- Health ---

type HealthResponse struct {
	Status    string                 `json:"status"`
	Service   string                 `json:"service"`
	Timestamp string                 `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Sources   map[string]interface{} `json:"sources"`
}

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	sources := map[string]interface{}{
		"usgs":     s.health.USGS.GetStatus(),
		"phivolcs": s.health.PHIVOLCS.GetStatus(),
	}

	status := "ok"
	if s.health.USGS.GetStatus().Status == "down" && s.health.PHIVOLCS.GetStatus().Status == "down" {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    status,
		Service:   "lindol-api",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(startTime).Round(time.Second).String(),
		Sources:   sources,
	})
}

// --- Status ---

type StatusResponse struct {
	TotalEarthquakes int     `json:"total_earthquakes"`
	LastEventTime    *string `json:"last_event_time,omitempty"`
	LastPollUSGS     *string `json:"last_poll_usgs,omitempty"`
	LastPollPhivolcs *string `json:"last_poll_phivolcs,omitempty"`
	Uptime           string  `json:"uptime"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	// Get total count
	var total int
	_ = s.eqService.DB().Conn.QueryRow("SELECT COUNT(*) FROM earthquakes").Scan(&total)

	resp := StatusResponse{
		TotalEarthquakes: total,
		Uptime:           time.Since(startTime).Round(time.Second).String(),
	}

	// Get last event time
	var lastTime string
	err := s.eqService.DB().Conn.QueryRow("SELECT event_time FROM earthquakes ORDER BY event_time DESC LIMIT 1").Scan(&lastTime)
	if err == nil {
		resp.LastEventTime = &lastTime
	}

	// Get last poll times from health tracker
	usgsStatus := s.health.USGS.GetStatus()
	if usgsStatus.LastSuccess != nil {
		resp.LastPollUSGS = usgsStatus.LastSuccess
	}
	phivolcsStatus := s.health.PHIVOLCS.GetStatus()
	if phivolcsStatus.LastSuccess != nil {
		resp.LastPollPhivolcs = phivolcsStatus.LastSuccess
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Earthquakes ---

type ListResponse struct {
	Data    interface{} `json:"data"`
	Total   int         `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
	HasMore bool        `json:"has_more"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func (s *Server) handleListEarthquakes(w http.ResponseWriter, r *http.Request) {
	filters := services.ListFilters{
		Limit:  20,
		Offset: 0,
	}

	q := r.URL.Query()

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid limit", Details: "Must be a positive integer (1-100)"})
			return
		}
		if n > 100 {
			n = 100
		}
		filters.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid offset", Details: "Must be a non-negative integer"})
			return
		}
		filters.Offset = n
	}
	if v := q.Get("minMagnitude"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid minMagnitude", Details: "Must be a number"})
			return
		}
		filters.MinMagnitude = &f
	}
	if v := q.Get("maxMagnitude"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid maxMagnitude", Details: "Must be a number"})
			return
		}
		filters.MaxMagnitude = &f
	}
	if v := q.Get("startDate"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid startDate", Details: "Use ISO 8601 (2006-01-02 or 2006-01-02T15:04:05Z)"})
			return
		}
		filters.StartDate = &t
	}
	if v := q.Get("endDate"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid endDate", Details: "Use ISO 8601 (2006-01-02 or 2006-01-02T15:04:05Z)"})
			return
		}
		// If just a date, include the whole day
		if len(v) == 10 {
			t = t.Add(24*time.Hour - time.Second)
		}
		filters.EndDate = &t
	}

	earthquakes, total, err := s.eqService.List(filters)
	if err != nil {
		s.logger.Error("Failed to list earthquakes", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
		return
	}

	if earthquakes == nil {
		writeJSON(w, http.StatusOK, ListResponse{
			Data:    []interface{}{},
			Total:   total,
			Limit:   filters.Limit,
			Offset:  filters.Offset,
			HasMore: false,
		})
		return
	}

	writeJSON(w, http.StatusOK, ListResponse{
		Data:    earthquakes,
		Total:   total,
		Limit:   filters.Limit,
		Offset:  filters.Offset,
		HasMore: filters.Offset+filters.Limit < total,
	})
}

func (s *Server) handleLatestEarthquake(w http.ResponseWriter, _ *http.Request) {
	eq, err := s.eqService.GetLatest()
	if err != nil {
		s.logger.Error("Failed to get latest earthquake", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
		return
	}

	if eq == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "No earthquakes recorded yet"})
		return
	}

	writeJSON(w, http.StatusOK, eq)
}

func (s *Server) handleGetEarthquake(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Missing earthquake ID"})
		return
	}

	eq, err := s.eqService.GetByID(id)
	if err != nil {
		s.logger.Error("Failed to get earthquake", "id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
		return
	}

	if eq == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Earthquake not found"})
		return
	}

	writeJSON(w, http.StatusOK, eq)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func parseDate(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized date format")
}
