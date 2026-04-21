package main

import (
	"sync"
	"time"
)

type ObservationStore struct {
	mu             sync.RWMutex
	observations   []Observation
	windowDuration time.Duration
}

func NewObservationStore(windowDuration time.Duration) *ObservationStore {
	store := &ObservationStore{
		observations:   []Observation{},
		windowDuration: windowDuration,
	}

	// Start cleanup thread in background
	go store.cleanupLoop()

	return store
}

func (s *ObservationStore) Add(obs Observation) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.observations = append(s.observations, obs)
	// s.cleanup()
}

func (s *ObservationStore) GetRecent() []Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix() * 1000
	recent := []Observation{}

	for _, obs := range s.observations {
		if obs.Timestamp >= cutoff {
			recent = append(recent, obs)
		}
	}
	return recent
}

func (s *ObservationStore) GetByTarget(targetID string) []Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix() * 1000
	recent := []Observation{}

	for _, obs := range s.observations {
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

	return len(s.observations)
}

func (s *ObservationStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.windowDuration).Unix() * 1000
	filtered := []Observation{}

	for _, obs := range s.observations {
		if obs.Timestamp >= cutoff {
			filtered = append(filtered, obs)
		}
	}

	s.observations = filtered
}

func (s *ObservationStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

type VerdictStore struct {
    mu      sync.RWMutex
    verdicts map[string]Verdict   // one verdict per namespace, overwritten each cycle
}

func NewVerdictStore() *VerdictStore {
	store := &VerdictStore{
		verdicts:   map[string]Verdict{},
	}

	return store
}

func (vs *VerdictStore) Set(namespace string, v Verdict) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.verdicts[namespace] = v
}

func (vs *VerdictStore) GetAll() []Verdict {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var allVerdicts []Verdict
	for _, ns_verdicts := range vs.verdicts {
		allVerdicts = append(allVerdicts, ns_verdicts)
	}

	return allVerdicts
}
