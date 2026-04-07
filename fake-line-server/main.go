package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type LINEMessageRequest struct {
	To       string        `json:"to"`
	Messages []LINEMessage `json:"messages"`
}

type LINEMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Fake LINE Chat Server...")

	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/v2/bot/message/push", handlePushMessage)
	http.HandleFunc("/health", handleHealth)

	port := "3000"
	log.Printf("Fake LINE Chat Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handlePushMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LINEMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received LINE Message:")
	log.Printf("  To: %s", req.To)
	for i, msg := range req.Messages {
		log.Printf("  Message %d:", i+1)
		log.Printf("    Type: %s", msg.Type)
		log.Printf("    Text: %s", msg.Text)
	}

	w.Header().Set("Content-Type", "application/json")

	if rand.Intn(10) == 0 {
		log.Printf("Returning 500 Internal Server Error (random failure for testing)")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal Server Error (random failure)",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Message received",
		"to":      req.To,
		"count":   len(req.Messages),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Status:  time.Now().Format(time.RFC3339),
		Service: "fake-line-server",
	})
}
