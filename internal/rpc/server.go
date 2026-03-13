package rpc

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"sunchain/internal/types"
)

type Handler interface {
	Health(ctx context.Context) (any, error)
	Validators(ctx context.Context) (any, error)
	LatestBlock(ctx context.Context) (any, error)
	SubscribeBlocks(buffer int) (int, <-chan types.Block)
	UnsubscribeBlocks(id int)
}

type Server struct {
	addr          string
	allowedOrigin string
	logger        *slog.Logger
	handle        Handler
	server        *http.Server
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

func New(addr string, handler Handler, allowedOrigin string, logger *slog.Logger) *Server {
	return &Server{addr: addr, handle: handler, allowedOrigin: allowedOrigin, logger: logger}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", s.serveRPC)
	mux.HandleFunc("/blocks", s.serveBlocksWS)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req Request
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
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

func (s *Server) serveBlocksWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		s.logger.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	subID, ch := s.handle.SubscribeBlocks(32)
	defer s.handle.UnsubscribeBlocks(subID)

	if latest, err := s.handle.LatestBlock(r.Context()); err == nil {
		if block, ok := latest.(types.Block); ok && block.Height > 0 {
			if err := wsWriteJSON(conn, block); err != nil {
				return
			}
		}
	}

	for block := range ch {
		if err := wsWriteJSON(conn, block); err != nil {
			s.logger.Debug("websocket write failed", "error", err)
			return
		}
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	if s.allowedOrigin == "" || s.allowedOrigin == "*" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return strings.EqualFold(origin, s.allowedOrigin)
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") && !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, fmt.Errorf("missing upgrade connection header")
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("missing websocket upgrade header")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, fmt.Errorf("unsupported websocket version")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing websocket key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("http server does not support hijacking")
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack connection: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("clear hijacked connection deadline: %w", err)
	}

	accept := computeAcceptKey(key)
	if _, err := buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := buf.WriteString("Upgrade: websocket\r\n"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := buf.WriteString("Connection: Upgrade\r\n"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := buf.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n"); err != nil {
		conn.Close()
		return nil, err
	}
	if err := buf.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func wsWriteJSON(conn net.Conn, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return wsWriteTextFrame(conn, data)
}

func wsWriteTextFrame(conn net.Conn, payload []byte) error {
	header := []byte{0x81}
	length := len(payload)

	switch {
	case length <= 125:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		header = append(header, ext...)
	default:
		header = append(header, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}

	writer := bufio.NewWriter(conn)
	if _, err := writer.Write(header); err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return writer.Flush()
}

func writeResult(w http.ResponseWriter, id any, result any, err error) {
	if err != nil {
		writeRPCError(w, id, -32000, err.Error())
		return
	}
	resp := Response{JSONRPC: "2.0", Result: result, ID: id}
	writeJSON(w, resp)
}

func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := Response{JSONRPC: "2.0", Error: &Error{Code: code, Message: message}, ID: id}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
