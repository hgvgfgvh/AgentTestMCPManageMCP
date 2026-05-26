// Package registry 持久化托管 MCP 登记表（3M 真源）。
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// LaunchSpec 子 MCP 启动方式（stdio）。
type LaunchSpec struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
}

// Record 单条托管 MCP。
type Record struct {
	MCPID       string     `json:"mcp_id"`
	Summary     string     `json:"summary"`
	Status      string     `json:"status"` // ready | pending | failed | degraded
	Source      string     `json:"source,omitempty"`
	Template    string     `json:"template,omitempty"`
	Requirement string     `json:"requirement,omitempty"`
	Launch      LaunchSpec `json:"launch,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

// OnChangeFunc Registry revision 变更回调（用于 Resource 更新 / 审计）。
type OnChangeFunc func(revision uint64)

// Store 线程安全 Registry + 磁盘持久化。
type Store struct {
	mu       sync.RWMutex
	path     string
	records  []Record
	revision atomic.Uint64
	onChange OnChangeFunc
}

// Open 加载或初始化 registry.json。
func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dataDir, "registry.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	if s.revision.Load() == 0 {
		s.revision.Store(1)
	}
	return s, nil
}

// SetOnChange 注册 revision 变更回调（非并发重入）。
func (s *Store) SetOnChange(fn OnChangeFunc) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var onDisk struct {
		Revision uint64   `json:"revision"`
		Records  []Record `json:"records"`
	}
	if err := json.Unmarshal(b, &onDisk); err != nil {
		return err
	}
	s.records = onDisk.Records
	if onDisk.Revision > 0 {
		s.revision.Store(onDisk.Revision)
	}
	return nil
}

func (s *Store) saveLocked() error {
	onDisk := struct {
		Revision uint64   `json:"revision"`
		Records  []Record `json:"records"`
	}{
		Revision: s.revision.Load(),
		Records:  s.records,
	}
	b, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) notifyChangeLocked() {
	rev := s.revision.Load()
	fn := s.onChange
	if fn != nil {
		go fn(rev)
	}
}

// Revision 当前 catalog_revision。
func (s *Store) Revision() uint64 {
	return s.revision.Load()
}

// List 返回副本。
func (s *Store) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out
}

// Get 按 mcp_id 查找。
func (s *Store) Get(mcpID string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.records {
		if r.MCPID == mcpID {
			return r, true
		}
	}
	return Record{}, false
}

// Upsert 写入并持久化；revision++。
func (s *Store) Upsert(rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i, r := range s.records {
		if r.MCPID == rec.MCPID {
			s.records[i] = rec
			found = true
			break
		}
	}
	if !found {
		s.records = append(s.records, rec)
	}
	s.revision.Add(1)
	if err := s.saveLocked(); err != nil {
		return err
	}
	s.notifyChangeLocked()
	return nil
}

// SetStatus 更新状态（如 degraded）。
func (s *Store) SetStatus(mcpID, status, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.MCPID == mcpID {
			s.records[i].Status = status
			s.records[i].LastError = lastErr
			s.revision.Add(1)
			if err := s.saveLocked(); err != nil {
				return err
			}
			s.notifyChangeLocked()
			return nil
		}
	}
	return fmt.Errorf("unknown mcp_id %q", mcpID)
}

// Remove 删除条目。
func (s *Store) Remove(mcpID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.records[:0]
	removed := false
	for _, r := range s.records {
		if r.MCPID == mcpID {
			removed = true
			continue
		}
		out = append(out, r)
	}
	if !removed {
		return fmt.Errorf("unknown mcp_id %q", mcpID)
	}
	s.records = out
	s.revision.Add(1)
	if err := s.saveLocked(); err != nil {
		return err
	}
	s.notifyChangeLocked()
	return nil
}
