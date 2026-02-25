package main

import (
	"encoding/json"
	"net/http"
)

type Server struct {
	store *ObservationStore
}

func NewServer(store *ObservationStore) *Server {
	return &Server{store: store}
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
	var obs InfraObservation

	err := json.NewDecoder(r.Body).Decode(&obs)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	s.store.Add(obs)
	w.WriteHeader(http.StatusCreated)

}

func (s *Server) handleGetObservations(w http.ResponseWriter, r *http.Request) {
	var observations []InfraObservation

	if targetID := r.URL.Query().Get("target_id"); targetID != "" {
		observations = s.store.GetByTarget(targetID)
	} else {
		observations = s.store.GetRecent()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(observations)
}
