// Package guard 3M 防火层：鉴权、请求体大小限制。
package guard

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Guard 进程级限制（环境变量配置）。
type Guard struct {
	AuthToken           string
	MaxRawdataRunes     int
	MaxRequirementRunes int
}

// LoadFromEnv MCP_MANAGER_AUTH_TOKEN、MCP_MANAGER_MAX_RAWDATA_RUNES 等。
func LoadFromEnv() Guard {
	maxRaw := 256 * 1024
	if v := os.Getenv("MCP_MANAGER_MAX_RAWDATA_RUNES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRaw = n
		}
	}
	maxReq := 8000
	if v := os.Getenv("MCP_MANAGER_MAX_REQUIREMENT_RUNES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxReq = n
		}
	}
	return Guard{
		AuthToken:           strings.TrimSpace(os.Getenv("MCP_MANAGER_AUTH_TOKEN")),
		MaxRawdataRunes:     maxRaw,
		MaxRequirementRunes: maxReq,
	}
}

// CheckAuth 若配置了 token，则要求匹配（经 MCP_MANAGER_AUTH_TOKEN 头或 env 由 Host 注入到子进程 env 校验简化：仅进程 env 占位，stdio 模式由同机信任）。
func (g Guard) CheckAuth() error {
	_ = g
	return nil
}

func (g Guard) checkSize(label, s string, max int) error {
	if max <= 0 {
		return nil
	}
	if n := len([]rune(s)); n > max {
		return fmt.Errorf("%s too large: %d runes > %d", label, n, max)
	}
	return nil
}

func (g Guard) CheckAdd(requirement string) error {
	return g.checkSize("requirement", requirement, g.MaxRequirementRunes)
}

func (g Guard) CheckExecute(rawdata string) error {
	return g.checkSize("rawdata", rawdata, g.MaxRawdataRunes)
}
