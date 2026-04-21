package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	// Initialize the observation store with a window duration of 5 minutes
	store := NewObservationStore(5 * time.Minute)

	// Initialize the verdict store
	verdictStore := NewVerdictStore()

	// Create the server with the observation store
	server := NewServer(store, verdictStore)

	fmt.Println("Registering HTTP handlers")

	// Set up the HTTP handler for observations
	http.HandleFunc("/observations", server.HandleObservations)
	http.HandleFunc("/verdicts", server.HandleVerdicts)

	fmt.Println("Starting verdict server on port 80")

	analyzer := &ConsensusDiscrepancyAnalyzer{}
	go func() {
		// ticker := time.NewTicker(1 * time.Minute)
		// 5 seconds ticker for local testing
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			recent := store.GetRecent()
			grouped := groupByNamespace(recent)
			infraByNS, meshByNS := splitByType(grouped)
			for ns := range unionKeys(infraByNS, meshByNS) {
				if ns == "" { continue }
				v := analyzer.Analyze(ns, infraByNS[ns], meshByNS[ns])
				verdictStore.Set(ns, v)
				fmt.Printf("[VERDICT] namespace=%s type=%s grayness_score=%.4f reason=%q\n",
					ns, v.VerdictType, v.Confidence, v.Reason)
			}
		}
	}()

	// Start the HTTP server on port 80
	http.ListenAndServe(":80", nil)
}
