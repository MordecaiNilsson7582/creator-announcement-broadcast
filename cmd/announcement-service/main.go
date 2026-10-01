package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/creator-announcement-broadcast/internal/announcement"
	"github.com/infrai-examples/creator-announcement-broadcast/internal/infrai"
)

func main() {
	publisher, err := infrai.NewPublisher(os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}
	channel := os.Getenv("CREATOR_CHANNEL")
	if channel == "" {
		channel = "creator-commerce"
	}
	broadcaster, err := announcement.NewBroadcaster(channel, publisher)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /announcements", handleAnnouncement(broadcaster))
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("announcement service listening on %s for channel %s", server.Addr, channel)
	log.Fatal(server.ListenAndServe())
}

func handleAnnouncement(broadcaster *announcement.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input announcement.Input
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		result, err := broadcaster.Broadcast(r.Context(), input)
		if err != nil {
			status := http.StatusBadRequest
			var apiErr *infrai.APIError
			if errors.As(err, &apiErr) {
				status = http.StatusBadGateway
				if apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
					status = apiErr.HTTPStatus
				}
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
