package services

import (
	"sync"
	"time"
)

// SourceStatus represents the health of an external data source.
type SourceStatus struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"` // "up", "down", "degraded"
	LastSuccess  *string   `json:"last_success,omitempty"`
	LastError    *string   `json:"last_error,omitempty"`
	LastErrorMsg *string   `json:"last_error_message,omitempty"`
	ErrorCount   int       `json:"error_count"`
	mu           sync.RWMutex
}

// HealthTracker tracks the health of external sources.
type HealthTracker struct {
	USGS     *SourceStatus
	PHIVOLCS *SourceStatus
}

// NewHealthTracker creates a new health tracker.
func NewHealthTracker() *HealthTracker {
	return &HealthTracker{
		USGS:     &SourceStatus{Name: "USGS", Status: "unknown"},
		PHIVOLCS: &SourceStatus{Name: "PHIVOLCS", Status: "unknown"},
	}
}

// RecordSuccess marks a source as healthy.
func (s *SourceStatus) RecordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = "up"
	now := time.Now().UTC().Format(time.RFC3339)
	s.LastSuccess = &now
	s.ErrorCount = 0
}

// RecordError marks a source as having an error.
func (s *SourceStatus) RecordError(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	now := time.Now().UTC().Format(time.RFC3339)
	s.LastError = &now
	s.LastErrorMsg = &errMsg

	if s.ErrorCount >= 3 {
		s.Status = "down"
	} else {
		s.Status = "degraded"
	}
}

// GetStatus returns a snapshot of the source status.
func (s *SourceStatus) GetStatus() SourceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SourceStatus{
		Name:         s.Name,
		Status:       s.Status,
		LastSuccess:  s.LastSuccess,
		LastError:    s.LastError,
		LastErrorMsg: s.LastErrorMsg,
		ErrorCount:   s.ErrorCount,
	}
}
