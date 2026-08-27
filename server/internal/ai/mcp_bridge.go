package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type MCPBridge struct {
	baseURL    string
	httpClient *http.Client
}

func NewMCPBridge() *MCPBridge {
	url := os.Getenv("MIKROTIK_MCP_URL")
	if url == "" {
		url = "http://127.0.0.1:8000"
	}
	return &MCPBridge{
		baseURL: strings.TrimSuffix(url, "/"),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// IsAvailable checks if the Bun-native mikrotik-mcp sidecar is alive and responding
func (b *MCPBridge) IsAvailable() bool {
	req, err := http.NewRequest("GET", b.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// CallMCPTool executes a specific tool on the Bun-native mikrotik-mcp sidecar
func (b *MCPBridge) CallMCPTool(ctx context.Context, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	reqPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("mcp-%d", time.Now().UnixNano()),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp call: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/mcp", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create mcp request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mcp response: %w", err)
	}

	var rpcResp struct {
		Result map[string]interface{} `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return map[string]interface{}{"raw": string(respBytes)}, nil
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	log.Printf("[MCP Bridge] Tool %s executed successfully on sidecar", toolName)
	return rpcResp.Result, nil
}
