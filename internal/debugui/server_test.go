package debugui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"AgentTestMCPManageMCP/internal/engine"
)

func TestDebugUI_ListAPI(t *testing.T) {
	eng := engine.NewStubEngine()
	h := &handler{eng: eng}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/list", h.handleList)

	req := httptest.NewRequest(http.MethodPost, "/api/list", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["managed"]; !ok {
		t.Fatalf("missing managed: %v", body)
	}
}

func TestDebugUI_DisabledEngine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Start(ctx, Config{Engine: nil, Addr: "127.0.0.1:0"})
}
