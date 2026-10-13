package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type anthropic struct {
	cfg Config
	hc  *http.Client
}

func (a *anthropic) Generate(ctx context.Context, r Request) (Response, error) {
	var msgs []map[string]any
	for _, m := range r.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, map[string]any{"role": "user", "content": m.Text})
		case "assistant":
			var blocks []map[string]any
			if m.Text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Text})
			}
			for _, c := range m.Calls {
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": json.RawMessage(orObj(c.Args))})
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": blocks})
		case "tool":
			var blocks []map[string]any
			for _, res := range m.Results {
				blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": res.CallID, "content": res.Content, "is_error": res.IsError})
			}
			msgs = append(msgs, map[string]any{"role": "user", "content": blocks})
		}
	}
	body := map[string]any{"model": a.cfg.Model, "max_tokens": maxTokens(r), "messages": msgs}
	if r.System != "" {
		body["system"] = r.System
	}
	if len(r.Tools) > 0 {
		var ts []map[string]any
		for _, t := range r.Tools {
			ts = append(ts, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema})
		}
		body["tools"] = ts
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL(a.cfg, "https://api.anthropic.com")+"/v1/messages", bytes.NewReader(b))
	req.Header.Set("x-api-key", a.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := a.hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return Response{}, httpError("Anthropic", resp.StatusCode, raw)
	}
	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, err
	}
	var res Response
	for _, c := range out.Content {
		switch c.Type {
		case "text":
			res.Text += c.Text
		case "tool_use":
			res.Calls = append(res.Calls, Call{ID: c.ID, Name: c.Name, Args: c.Input})
		}
	}
	return res, nil
}

func orObj(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return b
}
