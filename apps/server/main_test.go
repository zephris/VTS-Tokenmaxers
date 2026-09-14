package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vts-tokenmaxers/apps/server/internal/auth"
	"vts-tokenmaxers/apps/server/internal/domain"
	"vts-tokenmaxers/apps/server/internal/messaging"
	"vts-tokenmaxers/apps/server/internal/stego"
	"vts-tokenmaxers/apps/server/internal/store"
	"vts-tokenmaxers/apps/server/internal/summary"
	"vts-tokenmaxers/apps/server/internal/transcript"
)

func TestHealthMatchesSharedContract(t *testing.T) {
	app := newTestServer(t)
	recorder := request(t, app.routes(), http.MethodGet, "/api/health")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		OK            bool               `json:"ok"`
		Mode          string             `json:"mode"`
		Data          domain.ImportStats `json:"data"`
		Steganography stego.Status       `json:"steganography"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.Mode != "data-only" || response.Data.StationCount != 14 || response.Data.BroadcastCount != 300 || response.Steganography.Configured {
		t.Fatalf("health response = %#v", response)
	}
}

func TestOrcaPodPageIsServed(t *testing.T) {
	app := newTestServer(t)
	recorder := request(t, app.routes(), http.MethodGet, "/orca_pog/index.html")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Matilda Bay") {
		t.Fatalf("unexpected orca pod page: %s", recorder.Body.String())
	}
}

func TestStationRoutesReturnDatasetAndDecodedPathIDs(t *testing.T) {
	app := newTestServer(t)
	token := loginAsStation(t, app, "Outpost-Alpha")
	list := requestAs(t, app.routes(), token, http.MethodGet, "/api/stations")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d; body = %s", list.Code, list.Body.String())
	}
	var stations domain.StationsResponse
	if err := json.Unmarshal(list.Body.Bytes(), &stations); err != nil {
		t.Fatal(err)
	}
	if stations.AnalysisTimestamp != "2026-11-20 09:57" || len(stations.Stations) != 14 {
		t.Fatalf("stations response = %#v", stations)
	}

	detail := requestAs(t, app.routes(), token, http.MethodGet, "/api/stations/Marv%20Mail/broadcasts")
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d; body = %s", detail.Code, detail.Body.String())
	}
	var response domain.StationBroadcastsResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Station.SenderID != "Marv Mail" || len(response.Broadcasts) == 0 {
		t.Fatalf("station detail = %#v", response)
	}
	first := response.Broadcasts[0]
	if first.EncryptionStatus != domain.EncryptionPlain || first.CarrierText != nil || first.Label != "" {
		t.Fatalf("broadcast contract = %#v", first)
	}
	if strings.Contains(detail.Body.String(), `"label"`) {
		t.Fatalf("station detail leaks retrospective ground-truth labels: %s", detail.Body.String())
	}
}

func TestUnknownStationRouteReturns404(t *testing.T) {
	app := newTestServer(t)
	token := loginAsStation(t, app, "Outpost-Alpha")
	recorder := requestAs(t, app.routes(), token, http.MethodGet, "/api/stations/does-not-exist/broadcasts")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestDashboardAndIncidentRoutesUseDatabase(t *testing.T) {
	app := newTestServer(t)
	token := loginAsStation(t, app, "Outpost-Alpha")
	dashboard := requestAs(t, app.routes(), token, http.MethodGet, "/api/dashboard")
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d; body = %s", dashboard.Code, dashboard.Body.String())
	}
	var response domain.DashboardResponse
	if err := json.Unmarshal(dashboard.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AnalysisTimestamp != "2026-11-20 09:57" || len(response.RecentBroadcasts) != 20 {
		t.Fatalf("dashboard response = %#v", response)
	}
}

func TestBroadcastSearchAndOutpostEvidenceContracts(t *testing.T) {
	app := newTestServer(t)
	token := loginAsStation(t, app, "Outpost-Alpha")
	search := requestAs(t, app.routes(), token, http.MethodGet, "/api/broadcasts?q=Outpost-Delta&limit=1")
	if search.Code != http.StatusOK {
		t.Fatalf("search status = %d; body = %s", search.Code, search.Body.String())
	}
	var broadcasts domain.BroadcastsResponse
	if err := json.Unmarshal(search.Body.Bytes(), &broadcasts); err != nil {
		t.Fatal(err)
	}
	if broadcasts.Pagination.Total == 0 || broadcasts.Pagination.Limit != 1 || len(broadcasts.Broadcasts) != 1 {
		t.Fatalf("broadcast search = %#v", broadcasts)
	}
	if strings.Contains(search.Body.String(), `"label"`) {
		t.Fatalf("broadcast search leaks private labels: %s", search.Body.String())
	}

	detail := requestAs(t, app.routes(), token, http.MethodGet, "/api/outposts/Outpost-Delta")
	if detail.Code != http.StatusOK {
		t.Fatalf("outpost status = %d; body = %s", detail.Code, detail.Body.String())
	}
	var outpost domain.OutpostDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &outpost); err != nil {
		t.Fatal(err)
	}
	if outpost.Outpost.RiskLevel != domain.RiskCritical || len(outpost.Outpost.RiskReasons) == 0 || len(outpost.EvidenceTimeline) == 0 {
		t.Fatalf("outpost detail = %#v", outpost)
	}
}

func TestSteganographyTranscriptAvailableWhileModelDisabled(t *testing.T) {
	app := newTestServer(t)
	token := loginAsStation(t, app, "Outpost-Alpha")
	conversation := requestAs(t, app.routes(), token, http.MethodGet, "/api/steganography/conversations/demo?stationId=Outpost-Alpha")
	if conversation.Code != http.StatusOK {
		t.Fatalf("conversation status = %d; body = %s", conversation.Code, conversation.Body.String())
	}

	encode := requestBodyAs(t, app.routes(), token, http.MethodPost, "/api/steganography/encode", strings.NewReader(`{
		"conversationId":"demo","stationId":"Outpost-Alpha","sender":"Alpha",
		"secretPhrase":"a sufficiently long shared phrase","plaintext":"dispatch scout"
	}`))
	if encode.Code != http.StatusServiceUnavailable {
		t.Fatalf("encode status = %d; body = %s", encode.Code, encode.Body.String())
	}
}

func TestStationAuthenticationFlow(t *testing.T) {
	app := newTestServer(t)
	handler := app.routes()

	anonymous := request(t, handler, http.MethodGet, "/api/dashboard")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d; body = %s", anonymous.Code, anonymous.Body.String())
	}

	accounts := request(t, handler, http.MethodGet, "/api/auth/stations")
	if accounts.Code != http.StatusOK {
		t.Fatalf("accounts status = %d; body = %s", accounts.Code, accounts.Body.String())
	}
	if !strings.Contains(accounts.Body.String(), `"username":"Outpost-Alpha"`) {
		t.Fatalf("station account list did not include Outpost-Alpha: %s", accounts.Body.String())
	}

	badLogin := requestBody(t, handler, http.MethodPost, "/api/auth/login", strings.NewReader(`{
		"username":"Outpost-Alpha","password":"wrong-password"
	}`))
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d; body = %s", badLogin.Code, badLogin.Body.String())
	}

	token := loginAsStation(t, app, "Outpost-Alpha")
	session := requestAs(t, handler, token, http.MethodGet, "/api/auth/session")
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d; body = %s", session.Code, session.Body.String())
	}
	if !strings.Contains(session.Body.String(), `"stationId":"Outpost-Alpha"`) {
		t.Fatalf("session response = %s", session.Body.String())
	}

	logout := requestAs(t, handler, token, http.MethodPost, "/api/auth/logout")
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d; body = %s", logout.Code, logout.Body.String())
	}
	expired := requestAs(t, handler, token, http.MethodGet, "/api/auth/session")
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d; body = %s", expired.Code, expired.Body.String())
	}
}

func newTestServer(t *testing.T) *server {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate server test file")
	}
	datasetDir := filepath.Clean(filepath.Join(filepath.Dir(filename), "data/source/sunken-garden-and-cs-building.zip"))
	dataStore, stats, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "outposts.db"), datasetDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	transcripts := transcript.New(dataStore.Database())
	if err := transcripts.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	authService := auth.New(dataStore.Database(), auth.Config{DefaultPassword: "station-demo"})
	if _, err := authService.EnsureStationAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	codec := stego.DisabledService{}
	return &server{
		store: dataStore, auth: authService, stego: codec, messaging: messaging.New(codec, transcripts), analyzer: summary.StubAnalyzer{},
		importStats: stats, mode: "data-only",
	}
}

func request(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	return requestBody(t, handler, method, target, nil)
}

func requestBody(t *testing.T, handler http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	return requestBodyAs(t, handler, "", method, target, body)
}

func requestAs(t *testing.T, handler http.Handler, token, method, target string) *httptest.ResponseRecorder {
	return requestBodyAs(t, handler, token, method, target, nil)
}

func requestBodyAs(t *testing.T, handler http.Handler, token, method, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, body)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func loginAsStation(t *testing.T, app *server, username string) string {
	t.Helper()
	recorder := requestBody(t, app.routes(), http.MethodPost, "/api/auth/login", strings.NewReader(fmt.Sprintf(`{
		"username":%q,"password":"station-demo"
	}`, username)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var response auth.LoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Token == "" || response.Account.StationID != username {
		t.Fatalf("login response = %#v", response)
	}
	return response.Token
}
