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
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			recent := store.GetRecent()
			infraByDep := groupInfraByDeployment(recent)
			meshByDep := groupMeshByDeployment(recent)
			for depKey := range unionKeys(infraByDep, meshByDep) {
				if depKey == "" || depKey == "/" {
					continue
				}
				v := analyzer.Analyze(depKey, infraByDep[depKey], meshByDep[depKey])
				verdictStore.Set(depKey, v)
				fmt.Printf("[VERDICT] deployment=%s type=%s confidence=%.4f reason=%q\n",
					depKey, v.VerdictType, v.Confidence, v.Reason)
			}
		}
	}()

	// Start the HTTP server on port 80
	http.ListenAndServe(":80", nil)
}
