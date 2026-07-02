package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

type TagsResponse struct {
	Models []Model `json:"models"`
}

type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
	Size       int64     `json:"size,omitempty"`
	Digest     string    `json:"digest,omitempty"`
	Details    Details   `json:"details,omitempty"`
}

type Details struct {
	Format            string   `json:"format,omitempty"`
	Family            string   `json:"family,omitempty"`
	ParameterSize     string   `json:"parameter_size,omitempty"`
	QuantizationLevel string   `json:"quantization_level,omitempty"`
	Families          []string `json:"families,omitempty"`
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ollama returned %s", resp.Status)
	}
	return nil
}

func (c *Client) Tags(ctx context.Context) (TagsResponse, error) {
	var tags TagsResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return tags, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return tags, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return tags, fmt.Errorf("ollama returned %s", resp.Status)
	}
	return tags, json.NewDecoder(resp.Body).Decode(&tags)
}

func (c *Client) Proxy(ctx context.Context, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func CopyResponse(w http.ResponseWriter, resp *http.Response) error {
	defer resp.Body.Close()
	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err := io.Copy(w, resp.Body)
	return err
}
