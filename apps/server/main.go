package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vts-tokenmaxers/apps/server/internal/auth"
	"vts-tokenmaxers/apps/server/internal/config"
	"vts-tokenmaxers/apps/server/internal/domain"
	"vts-tokenmaxers/apps/server/internal/messaging"
	"vts-tokenmaxers/apps/server/internal/stego"
	"vts-tokenmaxers/apps/server/internal/store"
	"vts-tokenmaxers/apps/server/internal/summary"
	"vts-tokenmaxers/apps/server/internal/transcript"
)

type server struct {
	store       *store.Store
	auth        *auth.Service
	stego       stego.Service
	messaging   *messaging.Service
	analyzer    summary.Analyzer
	importStats domain.ImportStats
	mode        string
}

type incidentSummaryRequest struct {
	SenderID string `json:"senderId"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type healthResponse struct {
	OK            bool               `json:"ok"`
	Mode          string             `json:"mode"`
	Data          domain.ImportStats `json:"data"`
	Steganography stego.Status       `json:"steganography"`
}

type stationAccountsResponse struct {
	Accounts []auth.StationAccount `json:"accounts"`
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
	databasePath := envString("DATABASE_PATH", "./data/silent-outposts.db")
	datasetDir := envString("DATASET_ARCHIVE_PATH", envString("DATASET_DIR", "./data/source/sunken-garden-and-cs-building.zip"))
	dataStore, importStats, err := store.Open(ctx, databasePath, datasetDir)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := dataStore.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	stegoService, err := stego.NewFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := stegoService.Close(); err != nil && ctx.Err() == nil {
			log.Printf("close conversation steganography model: %v", err)
		}
	}()
	transcriptStore := transcript.New(dataStore.Database())
	if err := transcriptStore.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	authService := auth.New(dataStore.Database(), auth.Config{
		DefaultPassword: envString("STATION_ACCOUNT_DEFAULT_PASSWORD", "station-demo"),
		SessionTTL:      time.Duration(envInt("STATION_SESSION_HOURS", 12)) * time.Hour,
	})
	seededAccounts, err := authService.EnsureStationAccounts(ctx)
	if err != nil {
		log.Fatal(err)
	}

	port := envInt("PORT", 3001)
	mode := "data-only"
	if stegoService.Status().Configured {
		mode = "full"
	}
	app := &server{
		store:       dataStore,
		auth:        authService,
		stego:       stegoService,
		messaging:   messaging.New(stegoService, transcriptStore),
		analyzer:    summary.StubAnalyzer{},
		importStats: importStats,
		mode:        mode,
	}

	handler := withCORS(app.routes(), parseAllowedOrigins(envString("CORS_ALLOWED_ORIGINS", "http://localhost:5173")))
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	status := stegoService.Status()
	if status.Configured {
		log.Printf("Conversation steganography model ready: %s", status.ModelFingerprint)
	} else {
		log.Printf("Conversation steganography is disabled; set STEGANOGRAPHY_ENABLED=true in %s to enable it", envFile)
	}
	log.Printf("Imported %d stations and %d broadcasts from %s", importStats.StationCount, importStats.BroadcastCount, datasetDir)
	if seededAccounts > 0 {
		log.Printf("Seeded %d station login accounts", seededAccounts)
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
	mux.HandleFunc("GET /api/auth/stations", s.stationAccounts)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/session", s.requireAuth(s.session))
	mux.HandleFunc("POST /api/auth/logout", s.requireAuth(s.logout))
	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.dashboard))
	mux.HandleFunc("GET /api/stations", s.requireAuth(s.stations))
	mux.HandleFunc("GET /api/stations/{senderId}/broadcasts", s.requireAuth(s.stationBroadcasts))
	mux.HandleFunc("GET /api/broadcasts", s.requireAuth(s.broadcasts))
	mux.HandleFunc("GET /api/outposts/{senderId}", s.requireAuth(s.outpost))
	mux.HandleFunc("POST /api/ai/incident-summary", s.requireAuth(s.incidentSummary))
	mux.HandleFunc("GET /api/steganography/conversations/{conversationId}", s.requireAuth(s.steganographyConversation))
	mux.HandleFunc("POST /api/steganography/encode", s.requireAuth(s.encodeSteganography))
	mux.HandleFunc("POST /api/steganography/decode", s.requireAuth(s.decodeSteganography))
	return mux
}

func (s *server) stationAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.auth.StationAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Station accounts are unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, stationAccountsResponse{Accounts: accounts})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var request auth.LoginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	response, err := s.auth.Login(r.Context(), request.Username, request.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "Station username or password is incorrect.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Station sign-in is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	token, _ := bearerToken(r)
	response, err := s.auth.Authenticate(r.Context(), token)
	if errors.Is(err, auth.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "Station session has expired.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Station session is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	token, _ := bearerToken(r)
	if err := s.auth.Logout(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, "Station sign-out is unavailable.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Sign in with a station account to continue.")
			return
		}
		session, err := s.auth.Authenticate(r.Context(), token)
		if errors.Is(err, auth.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "Station session has expired.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Station session is unavailable.")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithAccount(r.Context(), session.Account)))
	}
}

func (s *server) broadcasts(w http.ResponseWriter, r *http.Request) {
	page, err := positiveQueryInt(r, "page", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := positiveQueryInt(r, "limit", 50)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := s.store.Broadcasts(r.Context(), domain.BroadcastFilter{
		SenderID: r.URL.Query().Get("senderId"), Location: r.URL.Query().Get("location"),
		Type:             domain.BroadcastType(r.URL.Query().Get("type")),
		CrossCheckStatus: domain.CrossCheckStatus(r.URL.Query().Get("crossCheckStatus")),
		Query:            r.URL.Query().Get("q"), Page: page, Limit: limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Broadcast data is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) outpost(w http.ResponseWriter, r *http.Request) {
	senderID := strings.TrimSpace(r.PathValue("senderId"))
	if senderID == "" || len(senderID) > 100 {
		writeError(w, http.StatusBadRequest, "A valid outpost senderId is required.")
		return
	}
	response, err := s.store.Outpost(r.Context(), senderID)
	if errors.Is(err, store.ErrStationNotFound) {
		writeError(w, http.StatusNotFound, "Outpost not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Outpost evidence is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		OK: true, Mode: s.mode, Data: s.importStats, Steganography: s.stego.Status(),
	})
}

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := s.store.Dashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dashboard data is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *server) stations(w http.ResponseWriter, r *http.Request) {
	response, err := s.store.Stations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Station data is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) stationBroadcasts(w http.ResponseWriter, r *http.Request) {
	senderID := strings.TrimSpace(r.PathValue("senderId"))
	if senderID == "" || len(senderID) > 100 {
		writeError(w, http.StatusBadRequest, "A valid station senderId is required.")
		return
	}
	response, err := s.store.StationBroadcasts(r.Context(), senderID)
	if errors.Is(err, store.ErrStationNotFound) {
		writeError(w, http.StatusNotFound, "Station not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Station broadcasts are unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) incidentSummary(w http.ResponseWriter, r *http.Request) {
	var request incidentSummaryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	senderID := strings.TrimSpace(request.SenderID)
	if senderID == "" || len(senderID) > 100 {
		writeError(w, http.StatusBadRequest, "A valid senderId is required.")
		return
	}

	evidence, err := s.store.IncidentEvidence(r.Context(), senderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Incident evidence is unavailable.")
		return
	}
	response, err := s.analyzer.Analyze(r.Context(), senderID, evidence)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Incident analysis is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) encodeSteganography(w http.ResponseWriter, r *http.Request) {
	var request messaging.EncodeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	response, err := s.messaging.Encode(r.Context(), request)
	if writeSteganographyError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) decodeSteganography(w http.ResponseWriter, r *http.Request) {
	var request messaging.DecodeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}

	response, err := s.messaging.Decode(r.Context(), request)
	if writeSteganographyError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) steganographyConversation(w http.ResponseWriter, r *http.Request) {
	response, err := s.messaging.Conversation(
		r.Context(), strings.TrimSpace(r.PathValue("conversationId")), strings.TrimSpace(r.URL.Query().Get("stationId")),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func writeSteganographyError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, stego.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "Conversation steganography is not configured on this server.")
	case errors.Is(err, transcript.ErrSequenceConflict):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
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

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	const prefix = "bearer "
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

func withCORS(next http.Handler, allowedOrigins map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, allowed := allowedOrigins[origin]; origin != "" && allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			result[origin] = struct{}{}
		}
	}
	return result
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
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

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
