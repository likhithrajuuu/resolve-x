package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type gemini struct {
	cfg Config
	hc  *http.Client
}

// geminiSchema strips JSON Schema keywords Gemini's function declarations reject.
func geminiSchema(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if k == "additionalProperties" || k == "$schema" {
				continue
			}
			out[k] = geminiSchema(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = geminiSchema(x[i])
		}
		return out
	}
	return v
}

func (g *gemini) Generate(ctx context.Context, r Request) (Response, error) {
	var contents []map[string]any
	for _, m := range r.Messages {
		switch m.Role {
		case "user":
			contents = append(contents, map[string]any{"role": "user", "parts": []map[string]any{{"text": m.Text}}})
		case "assistant":
			var parts []map[string]any
			if m.Text != "" {
				parts = append(parts, map[string]any{"text": m.Text})
			}
			for _, c := range m.Calls {
				parts = append(parts, map[string]any{"functionCall": map[string]any{"name": c.Name, "args": json.RawMessage(orObj(c.Args))}})
			}
			contents = append(contents, map[string]any{"role": "model", "parts": parts})
		case "tool":
			var parts []map[string]any
			for _, res := range m.Results {
				parts = append(parts, map[string]any{"functionResponse": map[string]any{"name": res.Name, "response": map[string]any{"result": res.Content}}})
			}
			contents = append(contents, map[string]any{"role": "user", "parts": parts})
		}
	}
	body := map[string]any{"contents": contents, "generationConfig": map[string]any{"maxOutputTokens": maxTokens(r)}}
	if r.System != "" {
		body["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": r.System}}}
	}
	if len(r.Tools) > 0 {
		var decls []map[string]any
		for _, t := range r.Tools {
			decls = append(decls, map[string]any{"name": t.Name, "description": t.Description, "parameters": geminiSchema(t.Schema)})
		}
		body["tools"] = []map[string]any{{"functionDeclarations": decls}}
	}
	b, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL(g.cfg, "https://generativelanguage.googleapis.com"), url.PathEscape(g.cfg.Model))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	req.Header.Set("x-goog-api-key", g.cfg.APIKey) // header, not query string, so keys stay out of logs
	req.Header.Set("content-type", "application/json")
	resp, err := g.hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return Response{}, httpError("Gemini", resp.StatusCode, raw)
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, err
	}
	if len(out.Candidates) == 0 {
		return Response{}, fmt.Errorf("Gemini returned no candidates (the prompt may have been blocked)")
	}
	var res Response
	for i, p := range out.Candidates[0].Content.Parts {
		res.Text += p.Text
		if p.FunctionCall != nil { // Gemini has no call ids; synthesise stable ones
			res.Calls = append(res.Calls, Call{ID: fmt.Sprintf("call_%d", i), Name: p.FunctionCall.Name, Args: p.FunctionCall.Args})
		}
	}
	return res, nil
}
