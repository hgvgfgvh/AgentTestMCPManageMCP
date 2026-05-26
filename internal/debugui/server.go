// Package debugui 提供 3M 三工具的本地 Web 调试台（与 MCP 工具共用 engine.Engine）。
package debugui

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"AgentTestMCPManageMCP/internal/engine"
)

//go:embed web/*
var webFS embed.FS

// Config 调试 HTTP 服务配置。
type Config struct {
	Addr   string // 监听地址，默认 127.0.0.1:18094
	Engine engine.Engine
}

// Start 在后台 goroutine 启动 Web UI；ctx 取消时关闭服务。
func Start(ctx context.Context, cfg Config) {
	if cfg.Engine == nil {
		return
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = "127.0.0.1:18094"
	}

	mux := http.NewServeMux()
	h := &handler{eng: cfg.Engine}
	mux.HandleFunc("GET /", h.serveIndex)
	mux.HandleFunc("POST /api/list", h.handleList)
	mux.HandleFunc("POST /api/add", h.handleAdd)
	mux.HandleFunc("POST /api/execute", h.handleExecute)

	srv := &http.Server{
		Addr:              addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	go func() {
		log.Printf("[mcp-manager-mcp] debug WebUI http://%s/", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[mcp-manager-mcp] debug WebUI: %v", err)
		}
	}()
}

type handler struct {
	eng engine.Engine
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	b, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (h *handler) handleList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorrelationID string `json:"correlation_id"`
	}
	_ = readJSON(r, &body)
	out := h.eng.ListManaged(r.Context(), engine.ListInput{CorrelationID: body.CorrelationID})
	writeToolJSON(w, out)
}

func (h *handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Requirement   string `json:"requirement"`
		Constraints   string `json:"constraints"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.Requirement) == "" {
		writeErr(w, http.StatusBadRequest, "requirement is required")
		return
	}
	out := h.eng.AddManaged(r.Context(), engine.AddInput{
		Requirement:   body.Requirement,
		Constraints:   body.Constraints,
		CorrelationID: body.CorrelationID,
	})
	writeToolJSON(w, out)
}

func (h *handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MCPID         string `json:"mcp_id"`
		StepGoal      string `json:"step_goal"`
		Rawdata       string `json:"rawdata"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.MCPID) == "" || strings.TrimSpace(body.StepGoal) == "" || strings.TrimSpace(body.Rawdata) == "" {
		writeErr(w, http.StatusBadRequest, "mcp_id, step_goal, rawdata are required")
		return
	}
	out := h.eng.ExecuteStep(r.Context(), engine.ExecuteInput{
		MCPID:         body.MCPID,
		StepGoal:      body.StepGoal,
		Rawdata:       body.Rawdata,
		CorrelationID: body.CorrelationID,
	})
	writeToolJSON(w, out)
}

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	if len(bytesTrim(b)) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func writeToolJSON(w http.ResponseWriter, jsonText string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var v any
	if err := json.Unmarshal([]byte(jsonText), &v); err != nil {
		_, _ = w.Write([]byte(jsonText))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
