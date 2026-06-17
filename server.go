package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"jacobrlewis/startgg-interface/startgg"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type apiServer struct {
	client        startgg.SGGClient
	allowOrigin   string
	operatorToken string
}

type apiError struct {
	Error string `json:"error"`
}

type assignStationRequest struct {
	SetId     int `json:"set_id"`
	StationId int `json:"station_id"`
}

type setActionRequest struct {
	SetId    int  `json:"set_id"`
	WinnerId int  `json:"winner_id,omitempty"`
	IsDQ     bool `json:"is_dq,omitempty"`
}

func runServer(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	allowOrigin := fs.String("allow-origin", "*", "CORS allow-origin value")
	operatorToken := fs.String("operator-token", "", "operator token for mutation endpoints")
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	server := &apiServer{
		client:        client,
		allowOrigin:   *allowOrigin,
		operatorToken: firstNonEmpty(*operatorToken, os.Getenv("STARTGG_OPERATOR_TOKEN"), os.Getenv("OPERATOR_TOKEN")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/tournament/status", server.handleTournamentStatus)
	mux.HandleFunc("/api/sets", server.handleSets)
	mux.HandleFunc("/api/sets/call", server.handleSetCall)
	mux.HandleFunc("/api/sets/progress", server.handleSetProgress)
	mux.HandleFunc("/api/sets/report", server.handleSetReport)
	mux.HandleFunc("/api/stations", server.handleStations)
	mux.HandleFunc("/api/stations/assign", server.handleStationAssign)
	if err := server.mountStatic(mux); err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           requestLogger(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Fprintf(out, "startgg-interface server listening on http://%s\n", *addr)
	fmt.Fprintln(out, "endpoints:")
	fmt.Fprintln(out, "  GET /healthz")
	fmt.Fprintln(out, "  GET /api/tournament/status?slug=2xko-test-solo")
	fmt.Fprintln(out, "  GET /api/sets?phase_group=3353163&state=pending")
	fmt.Fprintln(out, "  GET /api/stations?tournament=923152")
	fmt.Fprintln(out, "  GET /")
	if server.operatorToken == "" {
		fmt.Fprintln(out, "mutation endpoints disabled: set --operator-token or STARTGG_OPERATOR_TOKEN")
	} else {
		fmt.Fprintln(out, "mutation endpoints enabled: Authorization: Bearer <operator-token>")
	}
	return httpServer.ListenAndServe()
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.onlyGET(w, r) {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *apiServer) handleTournamentStatus(w http.ResponseWriter, r *http.Request) {
	if !s.onlyGET(w, r) {
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		s.writeError(w, http.StatusBadRequest, errors.New("missing slug query parameter"))
		return
	}
	payload, err := recoverAPI(func() any {
		return tournamentStatusFromAPI(s.client.GetTournamentStatus(slug))
	})
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *apiServer) handleSets(w http.ResponseWriter, r *http.Request) {
	if !s.onlyGET(w, r) {
		return
	}
	phaseGroup, err := queryInt(r, "phase_group", "phaseGroup")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	page := optionalQueryInt(r, 1, "page")
	perPage := optionalQueryInt(r, 50, "per_page", "perPage")
	state := r.URL.Query().Get("state")
	if strings.TrimSpace(state) == "" {
		state = "all"
	}

	payload, err := recoverAPI(func() any {
		sets, _ := s.client.GetPhaseGroupSets(phaseGroup, page, perPage)
		rows, err := filterSetRows(sets, state)
		if err != nil {
			panic(err)
		}
		return rows
	})
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "invalid --state") {
			status = http.StatusBadRequest
		}
		s.writeError(w, status, err)
		return
	}
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *apiServer) handleSetCall(w http.ResponseWriter, r *http.Request) {
	if !s.onlyPOST(w, r) || !s.authorized(w, r) {
		return
	}
	var req setActionRequest
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if req.SetId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing set_id"))
		return
	}
	if err := s.client.MarkSetCalled(req.SetId); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	logMutation(r, "sets.call", "set", req.SetId)
	s.writeJSON(w, http.StatusOK, mutationOutput{Ok: true, Operation: "sets call", SetId: req.SetId})
}

func (s *apiServer) handleSetProgress(w http.ResponseWriter, r *http.Request) {
	if !s.onlyPOST(w, r) || !s.authorized(w, r) {
		return
	}
	var req setActionRequest
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if req.SetId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing set_id"))
		return
	}
	if err := s.client.MarkSetInProgress(req.SetId); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	logMutation(r, "sets.progress", "set", req.SetId)
	s.writeJSON(w, http.StatusOK, mutationOutput{Ok: true, Operation: "sets progress", SetId: req.SetId})
}

func (s *apiServer) handleSetReport(w http.ResponseWriter, r *http.Request) {
	if !s.onlyPOST(w, r) || !s.authorized(w, r) {
		return
	}
	var req setActionRequest
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if req.SetId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing set_id"))
		return
	}
	if req.WinnerId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing winner_id"))
		return
	}
	if err := s.client.ReportSet(req.SetId, req.WinnerId, req.IsDQ); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	logMutation(r, "sets.report", "set", req.SetId, "winner", req.WinnerId, "dq", req.IsDQ)
	s.writeJSON(w, http.StatusOK, mutationOutput{Ok: true, Operation: "sets report", SetId: req.SetId, WinnerId: req.WinnerId, IsDQ: req.IsDQ})
}

func (s *apiServer) handleStations(w http.ResponseWriter, r *http.Request) {
	if !s.onlyGET(w, r) {
		return
	}
	tournament, err := queryInt(r, "tournament", "tournament_id", "tournamentId")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	payload, err := recoverAPI(func() any {
		return stationRows(s.client.GetTournamentStations(tournament))
	})
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *apiServer) handleStationAssign(w http.ResponseWriter, r *http.Request) {
	if !s.onlyPOST(w, r) || !s.authorized(w, r) {
		return
	}
	var req assignStationRequest
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if req.SetId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing set_id"))
		return
	}
	if req.StationId == 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("missing station_id"))
		return
	}
	if err := s.client.AssignStation(req.SetId, req.StationId); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	logMutation(r, "stations.assign", "set", req.SetId, "station", req.StationId)
	s.writeJSON(w, http.StatusOK, mutationOutput{Ok: true, Operation: "stations assign", SetId: req.SetId, StationId: req.StationId})
}

func (s *apiServer) onlyGET(w http.ResponseWriter, r *http.Request) bool {
	return s.onlyMethod(w, r, http.MethodGet)
}

func (s *apiServer) onlyPOST(w http.ResponseWriter, r *http.Request) bool {
	return s.onlyMethod(w, r, http.MethodPost)
}

func (s *apiServer) onlyMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == http.MethodOptions {
		s.writeJSON(w, http.StatusNoContent, nil)
		return false
	}
	if r.Method != method {
		s.writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return false
	}
	return true
}

func (s *apiServer) authorized(w http.ResponseWriter, r *http.Request) bool {
	if s.operatorToken == "" {
		s.writeError(w, http.StatusForbidden, errors.New("mutation endpoints disabled: set --operator-token or STARTGG_OPERATOR_TOKEN"))
		return false
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	token := strings.TrimSpace(r.Header.Get("X-Operator-Token"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token = strings.TrimSpace(auth[len("Bearer "):])
	}
	if token != s.operatorToken {
		s.writeError(w, http.StatusUnauthorized, errors.New("invalid operator token"))
		return false
	}
	return true
}

func (s *apiServer) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return false
	}
	return true
}

func (s *apiServer) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if s.allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Operator-Token")
	}
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func (s *apiServer) writeError(w http.ResponseWriter, status int, err error) {
	s.writeJSON(w, status, apiError{Error: err.Error()})
}

func (s *apiServer) mountStatic(mux *http.ServeMux) error {
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	return nil
}

func queryInt(r *http.Request, names ...string) (int, error) {
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed == 0 {
			return 0, fmt.Errorf("invalid %s query parameter", name)
		}
		return parsed, nil
	}
	return 0, fmt.Errorf("missing %s query parameter", names[0])
}

func optionalQueryInt(r *http.Request, fallback int, names ...string) int {
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func recoverAPI(fn func() any) (payload any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return fn(), nil
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func logMutation(r *http.Request, operation string, fields ...any) {
	parts := []string{
		"MUTATION ok",
		"operation=" + operation,
		"remote=" + clientAddress(r),
	}
	for i := 0; i+1 < len(fields); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
	}
	log.Print(strings.Join(parts, " "))
}

func clientAddress(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
