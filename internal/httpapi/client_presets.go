package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"llama-wrangler/internal/config"
)

const clientAPIKeyPlaceholder = "<client-api-key>"

type clientPreset struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	Client            string                `json:"client"`
	Protocol          string                `json:"protocol"`
	BaseURL           string                `json:"base_url"`
	Model             string                `json:"model"`
	APIKeyPlaceholder string                `json:"api_key_placeholder"`
	Fields            []clientPresetField   `json:"fields"`
	Snippets          []clientPresetSnippet `json:"snippets"`
}

type clientPresetField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type clientPresetSnippet struct {
	Label    string `json:"label"`
	Language string `json:"language"`
	Body     string `json:"body"`
}

func (s *Server) clientPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.buildClientPresets(r))
}

func (s *Server) buildClientPresets(r *http.Request) []clientPreset {
	state := s.store.Snapshot()
	cfg := state.Config
	baseURL := requestBaseURL(r)
	openAIBaseURL := strings.TrimRight(baseURL, "/") + "/v1"
	defaultModel := firstConfiguredModelAlias(cfg)
	codeModel := preferredModelAlias(cfg, "local-code", defaultModel)
	fastModel := preferredModelAlias(cfg, "local-fast", defaultModel)

	return []clientPreset{
		{
			ID:                "cline",
			Name:              "Cline",
			Client:            "Cline",
			Protocol:          "OpenAI-compatible",
			BaseURL:           openAIBaseURL,
			Model:             codeModel,
			APIKeyPlaceholder: clientAPIKeyPlaceholder,
			Fields:            openAIFields(openAIBaseURL, codeModel),
			Snippets: []clientPresetSnippet{
				{
					Label:    "OpenAI-compatible provider",
					Language: "json",
					Body: prettyJSON(map[string]string{
						"provider": "openai-compatible",
						"baseUrl":  openAIBaseURL,
						"apiKey":   clientAPIKeyPlaceholder,
						"model":    codeModel,
					}),
				},
			},
		},
		{
			ID:                "continue",
			Name:              "Continue",
			Client:            "Continue",
			Protocol:          "OpenAI-compatible",
			BaseURL:           openAIBaseURL,
			Model:             codeModel,
			APIKeyPlaceholder: clientAPIKeyPlaceholder,
			Fields:            openAIFields(openAIBaseURL, codeModel),
			Snippets: []clientPresetSnippet{
				{
					Label:    ".continue/config.json model",
					Language: "json",
					Body: prettyJSON(map[string]interface{}{
						"models": []map[string]string{
							{
								"title":    fmt.Sprintf("Llama Wrangler %s", codeModel),
								"provider": "openai",
								"model":    codeModel,
								"apiBase":  openAIBaseURL,
								"apiKey":   clientAPIKeyPlaceholder,
							},
						},
					}),
				},
			},
		},
		{
			ID:                "open-webui",
			Name:              "Open WebUI",
			Client:            "Open WebUI",
			Protocol:          "OpenAI-compatible",
			BaseURL:           openAIBaseURL,
			Model:             fastModel,
			APIKeyPlaceholder: clientAPIKeyPlaceholder,
			Fields:            openAIFields(openAIBaseURL, fastModel),
			Snippets: []clientPresetSnippet{
				{
					Label:    "OpenAI connection",
					Language: "text",
					Body:     fmt.Sprintf("OpenAI API Base URL: %s\nAPI Key: %s\nModel: %s", openAIBaseURL, clientAPIKeyPlaceholder, fastModel),
				},
			},
		},
		{
			ID:                "openai-sdk",
			Name:              "OpenAI SDK",
			Client:            "Generic OpenAI SDK",
			Protocol:          "OpenAI-compatible",
			BaseURL:           openAIBaseURL,
			Model:             fastModel,
			APIKeyPlaceholder: clientAPIKeyPlaceholder,
			Fields:            openAIFields(openAIBaseURL, fastModel),
			Snippets: []clientPresetSnippet{
				{
					Label:    "JavaScript",
					Language: "javascript",
					Body: fmt.Sprintf(`import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.LLAMA_WRANGLER_API_KEY || "%s",
  baseURL: "%s",
});

const response = await client.chat.completions.create({
  model: "%s",
  messages: [{ role: "user", content: "Hello from Llama Wrangler" }],
});`, clientAPIKeyPlaceholder, openAIBaseURL, fastModel),
				},
			},
		},
	}
}

func openAIFields(baseURL, model string) []clientPresetField {
	return []clientPresetField{
		{Label: "Base URL", Value: baseURL},
		{Label: "Model", Value: model},
		{Label: "API key", Value: clientAPIKeyPlaceholder},
	}
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "localhost:11435"
	}
	return scheme + "://" + host
}

func firstConfiguredModelAlias(cfg config.Config) string {
	if cfg.Routing.DefaultModelAlias != "" {
		return cfg.Routing.DefaultModelAlias
	}
	for alias := range cfg.ModelAliases {
		if alias != "" {
			return alias
		}
	}
	return "local-fast"
}

func preferredModelAlias(cfg config.Config, preferred, fallback string) string {
	if _, ok := cfg.ModelAliases[preferred]; ok {
		return preferred
	}
	if fallback != "" {
		return fallback
	}
	return firstConfiguredModelAlias(cfg)
}

func prettyJSON(value interface{}) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "{}"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
