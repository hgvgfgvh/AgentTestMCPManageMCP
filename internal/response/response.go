// Package response 将工具结果编码为 DESIGN_INTENT §5 约定的 JSON 字符串（MCP 文本结果）。
package response

import (
	"encoding/json"
	"fmt"
)

// ManagedEntry list_managed_mcps 中单条托管 MCP（L1）。
type ManagedEntry struct {
	MCPID   string `json:"mcp_id"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	Source  string `json:"source,omitempty"`
}

// ListPayload list_managed_mcps 返回体。
type ListPayload struct {
	CatalogRevision uint64         `json:"catalog_revision"`
	Managed         []ManagedEntry `json:"managed"`
	Message         string         `json:"message,omitempty"`
	Phase           string         `json:"phase"`
}

// AddPayload add_managed_mcp 返回体（不得含 catalog / l1_summary，见 M4）。
type AddPayload struct {
	Accepted bool   `json:"accepted"`
	MCPID    string `json:"mcp_id,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Error    string `json:"error,omitempty"`
	Phase    string `json:"phase"`
}

// ExecutePayload execute_step 返回体。
type ExecutePayload struct {
	OK         bool   `json:"ok"`
	MCPID      string `json:"mcp_id"`
	ToolUsed   string `json:"tool_used,omitempty"`
	Payload    string `json:"payload,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Phase      string `json:"phase"`
}

func FormatList(p ListPayload) string {
	if p.Phase == "" {
		p.Phase = PhaseStub()
	}
	return mustJSON(p)
}

func FormatAdd(p AddPayload) string {
	if p.Phase == "" {
		p.Phase = PhaseStub()
	}
	return mustJSON(p)
}

func FormatExecute(p ExecutePayload) string {
	if p.Phase == "" {
		p.Phase = PhaseStub()
	}
	return mustJSON(p)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"encode: %s","phase":"%s"}`, err.Error(), PhaseStub())
	}
	return string(b)
}

// ForbiddenAddResponseKeys M4：add 响应不得出现的字段。
var ForbiddenAddResponseKeys = []string{
	"l1_summary", "managed_catalog", "catalog_revision", "managed",
}

// AddResponseHasForbiddenFields 用于契约测试。
func AddResponseHasForbiddenFields(jsonText string) (string, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return "", false
	}
	for _, k := range ForbiddenAddResponseKeys {
		if _, ok := raw[k]; ok {
			return k, true
		}
	}
	return "", false
}
