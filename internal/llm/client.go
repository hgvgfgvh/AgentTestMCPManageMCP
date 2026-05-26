// Package llm OpenAI 兼容 Chat（3M 内部 Agent 专用）。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client Chat Completions 客户端。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// Enabled 是否已配置 API（MCP_MANAGER_LLM_API_BASE）。
func Enabled() bool {
	_, ok := ConfigFromEnv()
	return ok
}

// ConfigFromEnv MCP_MANAGER_LLM_* / OPENAI_API_KEY。
func ConfigFromEnv() (Client, bool) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("MCP_MANAGER_LLM_API_BASE")), "/")
	if base == "" {
		return Client{}, false
	}
	key := os.Getenv("MCP_MANAGER_LLM_API_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	model := strings.TrimSpace(os.Getenv("MCP_MANAGER_LLM_MODEL"))
	if model == "" {
		model = "deepseek-chat"
	}
	timeout := 60 * time.Second
	if v := os.Getenv("MCP_MANAGER_LLM_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return Client{
		BaseURL: base,
		APIKey:  key,
		Model:   model,
		HTTP:    &http.Client{Timeout: timeout},
	}, true
}

type chatReq struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 单轮对话（system + user）。
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 60 * time.Second}
	}
	url := c.BaseURL + "/chat/completions"
	body, _ := json.Marshal(chatReq{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out chatResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("llm: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm: empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ChatJSON 要求模型只输出 JSON 对象（剥除 ``` 包裹）。
func (c *Client) ChatJSON(ctx context.Context, system, user string) (string, error) {
	raw, err := c.Chat(ctx, system, user)
	if err != nil {
		return "", err
	}
	return extractJSONObject(raw), nil
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			s = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
