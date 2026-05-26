// Package audit 3M 侧 JSONL 审计（默认不记录完整 payload）。
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger 追加 JSONL。
type Logger struct {
	mu   sync.Mutex
	path string
}

// Open dataDir/audit/mcp-manager.jsonl
func Open(dataDir string) (*Logger, error) {
	dir := filepath.Join(dataDir, "audit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Logger{path: filepath.Join(dir, "mcp-manager.jsonl")}, nil
}

// Log 写入一行。
func (l *Logger) Log(kind string, fields map[string]any) {
	if l == nil {
		return
	}
	row := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "kind": kind}
	for k, v := range fields {
		row[k] = v
	}
	b, err := json.Marshal(row)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
