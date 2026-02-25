package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	// Initialize the observation store with a window duration of 1 minute
	store := NewObservationStore(1 * time.Minute)

	// Create the server with the observation store
	server := NewServer(store)

	fmt.Println("Registering HTTP handlers")

	// Set up the HTTP handler for observations
	http.HandleFunc("/observations", server.HandleObservations)

	fmt.Println("Starting verdict server on port 80")

	// Start the HTTP server on port 80
	http.ListenAndServe(":80", nil)
}
