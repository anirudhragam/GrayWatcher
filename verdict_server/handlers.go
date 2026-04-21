package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	store        *ObservationStore
	verdictStore *VerdictStore
}

func NewServer(store *ObservationStore, verdictStore *VerdictStore) *Server {
	return &Server{store: store, verdictStore: verdictStore}
}

func (s *Server) HandleObservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePostObservation(w, r)
	case http.MethodGet:
		s.handleGetObservations(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePostObservation(w http.ResponseWriter, r *http.Request) {
	var obs Observation

	err := json.NewDecoder(r.Body).Decode(&obs)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	s.store.Add(obs)

	if obs.ObserverType == "infrastructure" {
		phase, _ := obs.Metrics["phase"].(string)
		cpu_utilization_percent, ok := getFloat(obs.Metrics, "cpu_utilization_percent")
		if !ok {
			cpu_utilization_percent = 0.0
		}
		memory_utilization_percent, ok := getFloat(obs.Metrics, "memory_utilization_percent")
		if !ok {
			memory_utilization_percent = 0.0
		}
		fmt.Printf("[OBS] infra  target=%-40s status=%-10s phase=%-12s  cpu=%.1f%% mem=%.1f%%\n", obs.TargetID, obs.Status, phase, cpu_utilization_percent, memory_utilization_percent)
	} else {
		// obs.ObserverType == "mesh"
		success_rate, ok := getFloat(obs.Metrics, "success_rate")
		if !ok {
			success_rate = 0.0
		}
		error_rate, ok := getFloat(obs.Metrics, "error_rate")
		if !ok {
			error_rate = 0.0
		}
		p99_latency_ms, ok := getFloat(obs.Metrics, "p99_latency_ms")
		if !ok {
			p99_latency_ms = 0.0
		}
		request_rate, ok := getFloat(obs.Metrics, "request_rate")
		if !ok {
			request_rate = 0.0
		}
		fmt.Printf("[OBS] mesh   target=%-40s status=%-10s success_rate=%.3f error_rate=%.3f p99=%.1fms request_rate=%.3f\n", obs.TargetID, obs.Status, success_rate, error_rate, p99_latency_ms, request_rate)
	}

	w.WriteHeader(http.StatusCreated)

}

func (s *Server) handleGetObservations(w http.ResponseWriter, r *http.Request) {
	var observations []Observation

	if targetID := r.URL.Query().Get("target_id"); targetID != "" {
		observations = s.store.GetByTarget(targetID)
	} else {
		observations = s.store.GetRecent()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(observations)
}

func (s *Server) HandleVerdicts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetVerdicts(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetVerdicts(w http.ResponseWriter, r *http.Request) {
	verdicts := s.verdictStore.GetAll()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(verdicts)
}
