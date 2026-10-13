package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// ---- OpenAI Responses API (GPT, o-series and Codex models) ----

type openaiResponses struct {
	cfg Config
	hc  *http.Client
}

func (o *openaiResponses) Generate(ctx context.Context, r Request) (Response, error) {
	var input []any
	body := map[string]any{"model": o.cfg.Model, "max_output_tokens": maxTokens(r)}
	if r.System != "" {
		body["instructions"] = r.System
	}
	if r.State != "" && len(r.Messages) > 0 && r.Messages[len(r.Messages)-1].Role == "tool" {
		// Continue the stored conversation: only the new tool results are sent.
		body["previous_response_id"] = r.State
		for _, res := range r.Messages[len(r.Messages)-1].Results {
			input = append(input, map[string]any{"type": "function_call_output", "call_id": res.CallID, "output": res.Content})
		}
	} else {
		for _, m := range r.Messages {
			switch m.Role {
			case "user", "assistant":
				if m.Text != "" {
					input = append(input, map[string]any{"role": m.Role, "content": m.Text})
				}
			}
		}
	}
	body["input"] = input
	if len(r.Tools) > 0 {
		var ts []map[string]any
		for _, t := range r.Tools {
			ts = append(ts, map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": t.Schema})
		}
		body["tools"] = ts
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL(o.cfg, "https://api.openai.com")+"/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	req.Header.Set("content-type", "application/json")
	resp, err := o.hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return Response{}, httpError("OpenAI", resp.StatusCode, raw)
	}
	var out struct {
		ID     string `json:"id"`
		Output []struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, err
	}
	res := Response{State: out.ID}
	for _, it := range out.Output {
		switch it.Type {
		case "message":
			for _, c := range it.Content {
				if c.Type == "output_text" {
					res.Text += c.Text
				}
			}
		case "function_call":
			res.Calls = append(res.Calls, Call{ID: it.CallID, Name: it.Name, Args: json.RawMessage(orObj(json.RawMessage(it.Arguments)))})
		}
	}
	return res, nil
}

// ---- Chat Completions (Ollama, vLLM, LM Studio, OpenRouter, Azure, LiteLLM …) ----

type chatCompletions struct {
	cfg Config
	hc  *http.Client
}

func (c *chatCompletions) Generate(ctx context.Context, r Request) (Response, error) {
	var msgs []map[string]any
	if r.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": r.System})
	}
	for _, m := range r.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, map[string]any{"role": "user", "content": m.Text})
		case "assistant":
			am := map[string]any{"role": "assistant", "content": m.Text}
			if len(m.Calls) > 0 {
				var tcs []map[string]any
				for _, call := range m.Calls {
					tcs = append(tcs, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": string(orObj(call.Args))}})
				}
				am["tool_calls"] = tcs
			}
			msgs = append(msgs, am)
		case "tool":
			for _, res := range m.Results {
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": res.CallID, "content": res.Content})
			}
		}
	}
	body := map[string]any{"model": c.cfg.Model, "messages": msgs, "max_tokens": maxTokens(r)}
	if len(r.Tools) > 0 {
		var ts []map[string]any
		for _, t := range r.Tools {
			ts = append(ts, map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Schema}})
		}
		body["tools"] = ts
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL(c.cfg, "")+"/chat/completions", bytes.NewReader(b))
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	req.Header.Set("content-type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return Response{}, httpError("The endpoint", resp.StatusCode, raw)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return Response{}, io.ErrUnexpectedEOF
	}
	m := out.Choices[0].Message
	res := Response{Text: m.Content}
	for _, tc := range m.ToolCalls {
		res.Calls = append(res.Calls, Call{ID: tc.ID, Name: tc.Function.Name, Args: json.RawMessage(orObj(json.RawMessage(tc.Function.Arguments)))})
	}
	return res, nil
}
