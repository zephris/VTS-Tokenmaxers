package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vts-tokenmaxers/apps/server/internal/config"
	"vts-tokenmaxers/apps/server/internal/stego"
	"vts-tokenmaxers/apps/server/internal/store"
	"vts-tokenmaxers/apps/server/internal/summary"
)

type server struct {
	store *store.Store
	stego stego.Service
}

type incidentSummaryRequest struct {
	SenderID string `json:"senderId"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	envFile := strings.TrimSpace(os.Getenv("ENV_FILE"))
	if envFile == "" {
		envFile = ".env"
	}
	if err := config.LoadDotEnv(envFile); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stegoService, err := stego.NewFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := stegoService.Close(); err != nil && ctx.Err() == nil {
			log.Printf("close conversation steganography model: %v", err)
		}
	}()

	port := envInt("PORT", 3001)
	app := &server{
		store: store.NewSeeded(),
		stego: stegoService,
	}

	handler := withCORS(app.routes())
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	status := stegoService.Status()
	if status.Configured {
		log.Printf("Conversation steganography model ready: %s", status.ModelFingerprint)
	} else {
		log.Printf("Conversation steganography is disabled; set STEGANOGRAPHY_MODEL_COMMAND in %s to enable it", envFile)
	}
	log.Printf("Silent Outposts API listening on http://localhost:%d", port)
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			log.Printf("shut down API server: %v", err)
		}
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/dashboard", s.dashboard)
	mux.HandleFunc("POST /api/ai/incident-summary", s.incidentSummary)
	mux.HandleFunc("POST /api/steganography/encode", s.encodeSteganography)
	mux.HandleFunc("POST /api/steganography/decode", s.decodeSteganography)
	return mux
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "steganography": s.stego.Status()})
}

func (s *server) dashboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Dashboard())
}

func (s *server) incidentSummary(w http.ResponseWriter, r *http.Request) {
	var request incidentSummaryRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	senderID := strings.TrimSpace(request.SenderID)
	if senderID == "" || len(senderID) > 100 {
		writeError(w, http.StatusBadRequest, "A valid senderId is required.")
		return
	}

	evidence := s.store.IncidentEvidence(senderID)
	writeJSON(w, http.StatusOK, summary.Incident(senderID, evidence))
}

func (s *server) encodeSteganography(w http.ResponseWriter, r *http.Request) {
	var request stego.EncodeRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	response, err := s.stego.Encode(r.Context(), request)
	if errors.Is(err, stego.ErrNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "Conversation steganography is not configured on this server.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) decodeSteganography(w http.ResponseWriter, r *http.Request) {
	var request stego.DecodeRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	response, err := s.stego.Decode(r.Context(), request)
	if errors.Is(err, stego.ErrNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "Conversation steganography is not configured on this server.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
