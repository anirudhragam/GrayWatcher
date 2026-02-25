package main

import (
	"sync"
	"time"
)

type ObservationStore struct {
	mu                sync.RWMutex
	infraObservations []InfraObservation
	windowDuration    time.Duration
}

func NewObservationStore(windowDuration time.Duration) *ObservationStore {
	store := &ObservationStore{
		infraObservations: []InfraObservation{},
		windowDuration:    windowDuration,
	}

	// Start cleanup thread in background
	go store.cleanupLoop()

	return store
}

func (s *ObservationStore) Add(obs InfraObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.infraObservations = append(s.infraObservations, obs)
	// s.cleanup()
}

func (s *ObservationStore) GetRecent() []InfraObservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix()
	recent := []InfraObservation{}

	for _, obs := range s.infraObservations {
		if obs.Timestamp >= cutoff {
			recent = append(recent, obs)
		}
	}
	return recent
}

func (s *ObservationStore) GetByTarget(targetID string) []InfraObservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix()
	recent := []InfraObservation{}

	for _, obs := range s.infraObservations {
		if obs.Timestamp >= cutoff {
			if obs.TargetID == targetID {
				recent = append(recent, obs)
			}
		}
	}

	return recent
}

func (s *ObservationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.infraObservations)
}

func (s *ObservationStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix()
	filtered := []InfraObservation{}

	for _, obs := range s.infraObservations {
		if obs.Timestamp >= cutoff {
			filtered = append(filtered, obs)
		}
	}

	s.infraObservations = filtered
}

func (s *ObservationStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}
