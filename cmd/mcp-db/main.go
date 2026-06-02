package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"
	"NovaQuant/internal/repository"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := repository.New(pool)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			write(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		resp := handle(ctx, repo, req)
		if req.ID != nil {
			write(resp)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func handle(ctx context.Context, repo *repository.Repository, req request) response {
	resp := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "novaquant-db",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}
	case "notifications/initialized":
		return resp
	case "tools/list":
		resp.Result = map[string]any{
			"tools": []map[string]any{
				{
					"name":        "db_health",
					"description": "Check NovaQuant database health.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
				},
				{
					"name":        "market_overview",
					"description": "Read current market overview cards from NovaQuant.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
				},
				{
					"name":        "symbol_analysis",
					"description": "Read latest analysis for a symbol and interval.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"symbol":   map[string]any{"type": "string"},
							"interval": map[string]any{"type": "string"},
						},
						"required": []string{"symbol"},
					},
				},
				{
					"name":        "etf_flows",
					"description": "Read ETF flow rows for BTC or ETH.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"asset": map[string]any{"type": "string"},
							"limit": map[string]any{"type": "number"},
						},
						"required": []string{"asset"},
					},
				},
			},
		}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: err.Error()}
			return resp
		}
		content, err := callTool(ctx, repo, params)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": content},
			},
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: fmt.Sprintf("method %s not found", req.Method)}
	}
	return resp
}

func callTool(ctx context.Context, repo *repository.Repository, params toolCallParams) (string, error) {
	switch params.Name {
	case "db_health":
		err := repo.Ping(ctx)
		if err != nil {
			return "", err
		}
		return marshal(map[string]any{"status": "ok", "checked_at": time.Now().UTC()}), nil
	case "market_overview":
		rows, err := repo.GetMarketOverview(ctx, nil)
		if err != nil {
			return "", err
		}
		return marshal(rows), nil
	case "symbol_analysis":
		symbol, _ := params.Arguments["symbol"].(string)
		interval, _ := params.Arguments["interval"].(string)
		if interval == "" {
			interval = "1h"
		}
		row, err := repo.GetLatestAnalysis(ctx, symbol, interval)
		if err != nil {
			return "", err
		}
		return marshal(row), nil
	case "etf_flows":
		asset, _ := params.Arguments["asset"].(string)
		limit := 10
		if raw, ok := params.Arguments["limit"].(float64); ok && raw > 0 {
			limit = int(raw)
		}
		rows, err := repo.ListETFFlows(ctx, asset, limit)
		if err != nil {
			return "", err
		}
		return marshal(rows), nil
	default:
		return "", fmt.Errorf("tool %s not found", params.Name)
	}
}

func marshal(v any) string {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return string(raw)
}

func write(resp response) {
	raw, _ := json.Marshal(resp)
	fmt.Println(string(raw))
}
