package install

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadHTTP 将 URL 内容下载到 destPath（仅 http/https，host 白名单可选）。
func DownloadHTTP(ctx context.Context, rawURL, destPath string, allowHosts []string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if len(allowHosts) > 0 && !hostAllowed(u.Hostname(), allowHosts) {
		return fmt.Errorf("host %q not in download allowlist", u.Hostname())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download http %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, 64<<20))
	return err
}

func hostAllowed(host string, allow []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, a := range allow {
		if strings.ToLower(strings.TrimSpace(a)) == host {
			return true
		}
	}
	return len(allow) == 0
}

// LaunchManifest 工作区 launch.json。
type LaunchManifest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

func ReadLaunchManifest(path string) (LaunchManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return LaunchManifest{}, err
	}
	var m LaunchManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return LaunchManifest{}, err
	}
	return m, nil
}

func WriteLaunchManifest(path string, m LaunchManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
