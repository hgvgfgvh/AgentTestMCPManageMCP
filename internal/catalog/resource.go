// Package catalog 对外暴露只读 catalog 资源（辅助 revision；主路径仍为 list_managed_mcps）。
package catalog

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const resourceURI = "mcp-manager://catalog"

// SnapshotProvider 提供 list_managed_mcps 同等 JSON 文本。
type SnapshotProvider interface {
	ListManagedJSON(ctx context.Context) string
}

// Resource MCP catalog 只读资源。
type Resource struct {
	mu       sync.RWMutex
	provider SnapshotProvider
	server   *mcp.Server
	body     []byte
}

// New 创建资源持有者。
func New(provider SnapshotProvider) *Resource {
	return &Resource{provider: provider}
}

// Register 注册到 MCP Server。
func (r *Resource) Register(server *mcp.Server) {
	r.server = server
	server.AddResource(&mcp.Resource{
		URI:         resourceURI,
		Name:        "managed-mcp-catalog",
		Description: "3M 托管 MCP L1 快照（JSON）；与 list_managed_mcps 一致",
		MIMEType:    "application/json",
	}, r.read)
	r.Refresh()
}

func (r *Resource) read(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	r.mu.RLock()
	body := string(r.body)
	r.mu.RUnlock()
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: resourceURI, MIMEType: "application/json", Text: body},
		},
	}, nil
}

// Refresh 重建并通知 resource 更新。
func (r *Resource) Refresh() {
	if r == nil || r.provider == nil {
		return
	}
	text := r.provider.ListManagedJSON(context.Background())
	r.mu.Lock()
	r.body = []byte(text)
	r.mu.Unlock()
	if r.server != nil {
		_ = r.server.ResourceUpdated(context.Background(), &mcp.ResourceUpdatedNotificationParams{
			URI: resourceURI,
		})
	}
}
