package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"AgentTestMCPManageMCP/internal/response"
)

// StubEngine Phase-0：内存 Registry 桩；不下载/不启子 MCP/不调用内部 LLM。
type StubEngine struct {
	mu       sync.RWMutex
	entries  []response.ManagedEntry
	revision atomic.Uint64
}

// NewStubEngine 预置一条 demo 托管项，便于 Host 联调 list。
func NewStubEngine() *StubEngine {
	e := &StubEngine{}
	e.entries = []response.ManagedEntry{
		{
			MCPID:   "demo-echo",
			Summary: "桩：回显 rawdata（无真实子 MCP）",
			Status:  "ready",
			Source:  "manual",
		},
	}
	e.revision.Store(1)
	return e
}

func (e *StubEngine) ListManaged(ctx context.Context, in ListInput) string {
	_ = ctx
	_ = in
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]response.ManagedEntry, len(e.entries))
	copy(out, e.entries)
	return response.FormatList(response.ListPayload{
		CatalogRevision: e.revision.Load(),
		Managed:         out,
		Message:         "stub registry snapshot",
	})
}

func (e *StubEngine) AddManaged(ctx context.Context, in AddInput) string {
	_ = ctx
	req := strings.TrimSpace(in.Requirement)
	if req == "" {
		return response.FormatAdd(response.AddPayload{
			Accepted: false,
			Status:   "failed",
			Message:  "requirement is required",
			Error:    "missing_requirement",
		})
	}
	id := stubMCPID(req)
	summary := stubSummary(req)
	entry := response.ManagedEntry{
		MCPID:   id,
		Summary: summary,
		Status:  "ready",
		Source:  "generated",
	}
	e.mu.Lock()
	if idx := findEntry(e.entries, id); idx >= 0 {
		e.entries[idx] = entry
	} else {
		e.entries = append(e.entries, entry)
	}
	e.revision.Add(1)
	e.mu.Unlock()

	return response.FormatAdd(response.AddPayload{
		Accepted: true,
		MCPID:    id,
		Status:   "ready",
		Message:  "stub: registered in memory only (no install pipeline)",
	})
}

func (e *StubEngine) ExecuteStep(ctx context.Context, in ExecuteInput) string {
	_ = ctx
	start := time.Now()
	mcpID := strings.TrimSpace(in.MCPID)
	if mcpID == "" {
		return response.FormatExecute(response.ExecutePayload{
			OK:    false,
			MCPID: mcpID,
			Error: "missing_mcp_id",
		})
	}
	if strings.TrimSpace(in.StepGoal) == "" {
		return response.FormatExecute(response.ExecutePayload{
			OK:    false,
			MCPID: mcpID,
			Error: "missing_step_goal",
		})
	}
	if strings.TrimSpace(in.Rawdata) == "" {
		return response.FormatExecute(response.ExecutePayload{
			OK:    false,
			MCPID: mcpID,
			Error: "missing_rawdata",
		})
	}

	e.mu.RLock()
	ent, ok := entryByID(e.entries, mcpID)
	e.mu.RUnlock()
	if !ok {
		return response.FormatExecute(response.ExecutePayload{
			OK:         false,
			MCPID:      mcpID,
			Error:      "unknown_mcp_id",
			DurationMS: time.Since(start).Milliseconds(),
		})
	}
	if ent.Status != "ready" {
		return response.FormatExecute(response.ExecutePayload{
			OK:         false,
			MCPID:      mcpID,
			Error:      "mcp_not_ready:" + ent.Status,
			DurationMS: time.Since(start).Milliseconds(),
		})
	}

	// 桩 payload：模拟子 MCP 原始 JSON 文本（原样承载 rawdata，不改写语义内容）。
	inner, _ := json.Marshal(map[string]string{
		"stub":      "true",
		"echo":      in.Rawdata,
		"step_goal": in.StepGoal,
	})
	return response.FormatExecute(response.ExecutePayload{
		OK:         true,
		MCPID:      mcpID,
		ToolUsed:   "stub_echo",
		Payload:    string(inner),
		DurationMS: time.Since(start).Milliseconds(),
	})
}

func stubMCPID(requirement string) string {
	h := sha256.Sum256([]byte(strings.ToLower(requirement)))
	return "mcp-" + hex.EncodeToString(h[:6])
}

func stubSummary(requirement string) string {
	r := []rune(requirement)
	if len(r) > 80 {
		requirement = string(r[:80]) + "…"
	}
	return fmt.Sprintf("桩托管：%s", requirement)
}

func findEntry(entries []response.ManagedEntry, id string) int {
	for i, e := range entries {
		if e.MCPID == id {
			return i
		}
	}
	return -1
}

func entryByID(entries []response.ManagedEntry, id string) (response.ManagedEntry, bool) {
	if i := findEntry(entries, id); i >= 0 {
		return entries[i], true
	}
	return response.ManagedEntry{}, false
}
