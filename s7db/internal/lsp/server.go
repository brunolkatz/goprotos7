package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

type Config struct {
	SchemaPath string
	NoSchema   bool
	LogPath    string
	LogLevel   string
	Debounce   time.Duration
}

type Server struct {
	config         Config
	logger         *slog.Logger
	mu             sync.Mutex
	docs           map[string]string
	lastDiag       map[string][]Diagnostic
	debounceTimers map[string]*time.Timer
}

func New(cfg Config) *Server {
	if cfg.Debounce <= 0 {
		cfg.Debounce = 150 * time.Millisecond
	}
	lvl := slog.LevelError
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "warn":
		lvl = slog.LevelWarn
	case "info":
		lvl = slog.LevelInfo
	case "debug":
		lvl = slog.LevelDebug
	}
	var logger *slog.Logger
	if cfg.LogPath != "" {
		f, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			logger = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: lvl}))
		} else {
			logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: lvl}))
		}
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: lvl}))
	}
	return &Server{
		config:         cfg,
		logger:         logger,
		docs:           map[string]string{},
		lastDiag:       map[string][]Diagnostic{},
		debounceTimers: map[string]*time.Timer{},
	}
}

func (s *Server) ServeStdio() error {
	return s.serveIO(os.Stdin, os.Stdout)
}

func (s *Server) ServeTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	return s.serveIO(conn, conn)
}

func (s *Server) serveIO(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		msg, err := readMessage(br)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return err
		}
		if msg == nil {
			continue
		}
		if err := s.handleMessage(msg, w); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func readMessage(br *bufio.Reader) (*rpcMessage, error) {
	var length int
	for {
		header, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if header == "\r\n" || header == "\n" {
			break
		}
		header = strings.TrimSuffix(header, "\r\n")
		header = strings.TrimSuffix(header, "\n")
		parts := strings.SplitN(header, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
			}
		}
	}
	if length <= 0 {
		return nil, nil
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, err
	}
	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Server) handleMessage(msg *rpcMessage, w io.Writer) error {
	if msg == nil {
		return nil
	}
	if msg.Method == "initialize" {
		return s.sendResponse(w, msg.ID, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": map[string]any{
					"openClose": true,
					"change":    1,
					"save":      true,
				},
				"hoverProvider": false,
				"completionProvider": map[string]any{
					"triggerCharacters": []string{},
				},
			},
			"positionEncoding": "utf-16",
			"serverInfo": map[string]any{
				"name":    "s7db-lsp",
				"version": "dev",
			},
		})
	}
	if msg.Method == "initialized" || msg.Method == "" {
		return nil
	}
	if msg.Method == "shutdown" {
		return s.sendResponse(w, msg.ID, nil)
	}
	if msg.Method == "exit" {
		return io.EOF
	}
	if msg.Method == "textDocument/didOpen" {
		return s.handleDidOpen(msg.Params, w)
	}
	if msg.Method == "textDocument/didChange" {
		return s.handleDidChange(msg.Params, w)
	}
	if msg.Method == "textDocument/didSave" {
		return s.handleDidSave(msg.Params, w)
	}
	if msg.Method == "textDocument/didClose" {
		return s.handleDidClose(msg.Params, w)
	}
	return nil
}

func (s *Server) handleDidOpen(raw json.RawMessage, w io.Writer) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	return s.storeTextAndPublish(p.TextDocument.URI, p.TextDocument.Text, w)
}

func (s *Server) handleDidChange(raw json.RawMessage, w io.Writer) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	text := ""
	if len(p.ContentChanges) > 0 {
		text = p.ContentChanges[len(p.ContentChanges)-1].Text
	}
	if text == "" {
		text = s.getDoc(p.TextDocument.URI)
	}
	if s.config.Debounce > 0 {
		s.mu.Lock()
		if t := s.debounceTimers[p.TextDocument.URI]; t != nil {
			t.Stop()
		}
		s.docs[p.TextDocument.URI] = text
		s.debounceTimers[p.TextDocument.URI] = time.AfterFunc(s.config.Debounce, func() {
			s.publishDiagnostics(w, p.TextDocument.URI, s.compileText(uriToFile(p.TextDocument.URI), text))
		})
		s.mu.Unlock()
		return nil
	}
	return s.storeTextAndPublish(p.TextDocument.URI, text, w)
}

func (s *Server) handleDidSave(raw json.RawMessage, w io.Writer) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	text := s.getDoc(p.TextDocument.URI)
	return s.storeTextAndPublish(p.TextDocument.URI, text, w)
}

func (s *Server) handleDidClose(raw json.RawMessage, w io.Writer) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	s.mu.Lock()
	if t := s.debounceTimers[p.TextDocument.URI]; t != nil {
		t.Stop()
		delete(s.debounceTimers, p.TextDocument.URI)
	}
	delete(s.docs, p.TextDocument.URI)
	delete(s.lastDiag, p.TextDocument.URI)
	s.mu.Unlock()
	return s.publishDiagnostics(w, p.TextDocument.URI, nil)
}

func (s *Server) storeTextAndPublish(uri, text string, w io.Writer) error {
	path := uriToFile(uri)
	s.mu.Lock()
	s.docs[uri] = text
	s.mu.Unlock()
	return s.publishDiagnostics(w, uri, s.compileText(path, text))
}

func (s *Server) getDoc(uri string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[uri]
}

func (s *Server) publishDiagnostics(w io.Writer, uri string, diags []Diagnostic) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]any{
			"uri":        uri,
			"diagnostics": diags,
		},
	}
	return sendMessage(w, msg)
}

func (s *Server) CheckFile(path string) ([]Diagnostic, error) {
	srcBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.compileText(path, string(srcBytes)), nil
}

func (s *Server) compileText(file, src string) []Diagnostic {
	if src == "" {
		return nil
	}
	var sch schema.Schema
	if !s.config.NoSchema {
		path := s.config.SchemaPath
		if path == "" {
			path = resolveAutoSchemaPath(file)
		}
		if path == "" {
			return []Diagnostic{diagFromSchemaFailure(file, "schema file not found", "pass -f or create .s7db/s7db.yml")}
		}
		loaded, err := schema.Load(path, nil)
		if err != nil {
			return []Diagnostic{diagFromSchemaFailure(file, fmt.Sprintf("schema load failed: %v", err), "pass -f or create .s7db/s7db.yml")}
		}
		sch = loaded
	}
	_, diags := simlang.Compile(file, src, sch)
	out := make([]Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, diagToLSP(src, d))
	}
	return out
}

func diagFromSchemaFailure(file, msg, hint string) Diagnostic {
	return Diagnostic{
		Range: Range{
			Start: Position{Line: 0, Character: 0},
			End:   Position{Line: 0, Character: 1},
		},
		Severity: 1,
		Source:   "s7db",
		Message:  msg + "; " + hint,
	}
}

func resolveAutoSchemaPath(simPath string) string {
	candidates := []string{}
	if simPath != "" {
		d := filepath.Dir(simPath)
		candidates = append(candidates,
			filepath.Join(d, ".s7db", "s7db.yml"),
			filepath.Join(d, ".s7db", "s7db.yaml"),
			filepath.Join(d, "s7db.yml"),
		)
	}
	wd, err := os.Getwd()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(wd, ".s7db", "s7db.yml"),
			filepath.Join(wd, ".s7db", "s7db.yaml"),
			filepath.Join(wd, "s7db.yml"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func uriToFile(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		parsed, err := url.Parse(uri)
		if err == nil {
			if parsed.Path != "" {
				return filepath.FromSlash(parsed.Path)
			}
		}
	}
	return uri
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

func diagToLSP(src string, d simlang.Diag) Diagnostic {
	line := d.Line
	if line <= 0 {
		line = 1
	}
	col := d.Col
	if col <= 0 {
		col = 1
	}
	lines := strings.Split(src, "\n")
	if line > len(lines) {
		line = len(lines)
	}
	lineText := ""
	if line > 0 && line <= len(lines) {
		lineText = lines[line-1]
	}
	runes := []rune(lineText)
	prefixLen := 0
	if col-1 > 0 && col-1 <= len(runes) {
		prefixLen = col - 1
	}
	if col-1 > len(runes) {
		prefixLen = len(runes)
	}
	startCharacter := utf16Len(runes[:prefixLen])
	endCharacter := startCharacter
	if prefixLen < len(runes) {
		endCharacter = startCharacter + utf16Len(runes[prefixLen:prefixLen+1])
	} else {
		endCharacter = startCharacter + 1
	}
	msg := d.Msg
	if d.Hint != "" {
		msg = msg + "; " + d.Hint
	}
	return Diagnostic{
		Range: Range{
			Start: Position{Line: line - 1, Character: startCharacter},
			End:   Position{Line: line - 1, Character: endCharacter},
		},
		Severity: 1,
		Source:   "s7db",
		Message:  msg,
	}
}

func utf16Len(r []rune) int {
	return len(utf16.Encode(r))
}

func (s *Server) sendResponse(w io.Writer, id any, result any) error {
	return sendMessage(w, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func sendMessage(w io.Writer, msg map[string]any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
