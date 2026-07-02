package hec

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/config"
)

type Client struct {
	cfg        config.SplunkHECConfig
	httpClient *http.Client
}

type Payload struct {
	Time       float64     `json:"time,omitempty"`
	Host       string      `json:"host,omitempty"`
	Source     string      `json:"source,omitempty"`
	Sourcetype string      `json:"sourcetype,omitempty"`
	Index      string      `json:"index,omitempty"`
	Event      interface{} `json:"event"`
}

func New(cfg config.SplunkHECConfig) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: 10 * time.Second, Transport: transport}}
}

func (c *Client) Send(eventType string, event interface{}) error {
	if !c.cfg.Enabled {
		return nil
	}
	if c.cfg.URL == "" || c.cfg.Token == "" {
		return fmt.Errorf("splunk HEC is enabled but URL or token is missing")
	}
	payload := Payload{
		Time:       float64(time.Now().UnixNano()) / float64(time.Second),
		Source:     c.cfg.Source,
		Sourcetype: sourcetype(c.cfg.Prefix, eventType),
		Index:      c.cfg.Index,
		Event:      event,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Splunk "+c.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("splunk HEC returned %s", resp.Status)
	}
	return nil
}

func sourcetype(prefix, eventType string) string {
	if prefix == "" {
		prefix = "llama_wrangler"
	}
	eventType = strings.TrimPrefix(eventType, prefix+":")
	return prefix + ":" + eventType
}
