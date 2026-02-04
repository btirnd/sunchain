package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Handler interface {
	Health(ctx context.Context) (any, error)
	Validators(ctx context.Context) (any, error)
	LatestBlock(ctx context.Context) (any, error)
}

type Server struct {
	addr   string
	logger *slog.Logger
	handle Handler
	server *http.Server
}

type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params,omitempty"`
	ID      any              `json:"id"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	ID      any    `json:"id"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(addr string, handler Handler, logger *slog.Logger) *Server {
	return &Server{
		addr:   addr,
		handle: handler,
		logger: logger,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", s.serveRPC)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("rpc shutdown failed", "error", err)
		}
	}()

	s.logger.Info("rpc listening", "addr", s.addr)
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("rpc listen: %w", err)
	}

	return nil
}

func (s *Server) serveRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, req.ID, -32700, "invalid JSON")
		return
	}

	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "invalid JSON-RPC version")
		return
	}

	ctx := r.Context()
	switch req.Method {
	case "getHealth":
		result, err := s.handle.Health(ctx)
		writeResult(w, req.ID, result, err)
	case "getValidators":
		result, err := s.handle.Validators(ctx)
		writeResult(w, req.ID, result, err)
	case "getLatestBlock":
		result, err := s.handle.LatestBlock(ctx)
		writeResult(w, req.ID, result, err)
	default:
		writeRPCError(w, req.ID, -32601, "method not found")
	}
}

func writeResult(w http.ResponseWriter, id any, result any, err error) {
	if err != nil {
		writeRPCError(w, id, -32000, err.Error())
		return
	}
	resp := Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	writeJSON(w, resp)
}

func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
