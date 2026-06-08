// Package solver implements a cert-manager DNS-01 webhook solver for PanelDNS.
package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	acmev1 "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const defaultAPIURL = "https://app.paneldns.com"

// solverConfig is the per-issuer configuration decoded from ChallengeRequest.Config.
type solverConfig struct {
	APIURL           string          `json:"apiUrl"`
	APITokenSecretRef secretKeySelector `json:"apiTokenSecretRef"`
}

type secretKeySelector struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// PanelDNSSolver implements webhook.Solver for PanelDNS.
type PanelDNSSolver struct {
	client *kubernetes.Clientset
}

// Name returns the solver name registered with cert-manager.
func (s *PanelDNSSolver) Name() string {
	return "paneldns"
}

// Initialize wires up the Kubernetes client for secret lookups.
func (s *PanelDNSSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("cert-manager-webhook-paneldns: failed to create k8s client: %w", err)
	}
	s.client = cl
	return nil
}

// Present creates the _acme-challenge TXT record.
func (s *PanelDNSSolver) Present(ch *acmev1.ChallengeRequest) error {
	cfg, token, err := s.loadConfig(ch)
	if err != nil {
		return err
	}

	zoneID, zoneName, err := findZone(cfg.APIURL, token, ch.ResolvedFQDN)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}

	fqdn := strings.TrimSuffix(ch.ResolvedFQDN, ".")
	recordName := strings.TrimSuffix(fqdn, "."+zoneName)

	return createRecord(cfg.APIURL, token, zoneID, recordName, ch.Key)
}

// CleanUp removes the _acme-challenge TXT record.
func (s *PanelDNSSolver) CleanUp(ch *acmev1.ChallengeRequest) error {
	cfg, token, err := s.loadConfig(ch)
	if err != nil {
		return err
	}

	zoneID, _, err := findZone(cfg.APIURL, token, ch.ResolvedFQDN)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}

	recordID, err := findRecord(cfg.APIURL, token, zoneID, ch.Key)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	if recordID == 0 {
		return nil
	}
	return deleteRecord(cfg.APIURL, token, zoneID, recordID)
}

// loadConfig decodes the solver config and resolves the API token from a k8s secret.
func (s *PanelDNSSolver) loadConfig(ch *acmev1.ChallengeRequest) (*solverConfig, string, error) {
	cfg := &solverConfig{APIURL: defaultAPIURL}
	if ch.Config != nil {
		if err := json.Unmarshal(ch.Config.Raw, cfg); err != nil {
			return nil, "", fmt.Errorf("paneldns: failed to decode solver config: %w", err)
		}
	}
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")

	// Resolve API token from k8s secret
	ref := cfg.APITokenSecretRef
	ns := ref.Namespace
	if ns == "" {
		ns = ch.ResourceNamespace
	}

	secret, err := s.client.CoreV1().Secrets(ns).Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("paneldns: failed to get secret %s/%s: %w", ns, ref.Name, err)
	}

	key := ref.Key
	if key == "" {
		key = "token"
	}
	tokenBytes, ok := secret.Data[key]
	if !ok {
		return nil, "", fmt.Errorf("paneldns: key %q not found in secret %s/%s", key, ns, ref.Name)
	}

	return cfg, strings.TrimSpace(string(tokenBytes)), nil
}

// ── PanelDNS API helpers ──────────────────────────────────────────────────────

var httpClient = &http.Client{Timeout: 30 * time.Second}

type zone struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type record struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func findZone(apiURL, token, fqdn string) (int, string, error) {
	labels := strings.Split(strings.TrimSuffix(fqdn, "."), ".")
	for i := 1; i < len(labels); i++ {
		candidate := strings.Join(labels[i:], ".")
		if candidate == "" {
			continue
		}
		var envelope struct {
			Data []zone `json:"data"`
		}
		if err := doRequest(apiURL, token, http.MethodGet,
			fmt.Sprintf("/api/v1/zones?name=%s", url.QueryEscape(candidate)),
			nil, &envelope); err != nil {
			return 0, "", err
		}
		for _, z := range envelope.Data {
			if z.Name == candidate {
				return z.ID, z.Name, nil
			}
		}
	}
	return 0, "", fmt.Errorf("zone not found for %q", fqdn)
}

func createRecord(apiURL, token string, zoneID int, name, content string) error {
	body := map[string]interface{}{
		"type":    "TXT",
		"name":    name,
		"content": content,
		"ttl":     60,
	}
	return doRequest(apiURL, token, http.MethodPost,
		fmt.Sprintf("/api/v1/zones/%d/records", zoneID), body, nil)
}

func findRecord(apiURL, token string, zoneID int, content string) (int, error) {
	var envelope struct {
		Data []record `json:"data"`
	}
	if err := doRequest(apiURL, token, http.MethodGet,
		fmt.Sprintf("/api/v1/zones/%d/records", zoneID), nil, &envelope); err != nil {
		return 0, err
	}
	for _, r := range envelope.Data {
		if r.Type == "TXT" && r.Content == content {
			return r.ID, nil
		}
	}
	return 0, nil
}

func deleteRecord(apiURL, token string, zoneID, recordID int) error {
	return doRequest(apiURL, token, http.MethodDelete,
		fmt.Sprintf("/api/v1/zones/%d/records/%d", zoneID, recordID), nil, nil)
}

func doRequest(apiURL, token, method, path string, body, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, apiURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	if result != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

// groupName returns GROUP_NAME env var or falls back to the compiled default.
func init() {
	if v := os.Getenv("GROUP_NAME"); v != "" {
		_ = v // used by cmd.RunWebhookServer via env
	}
}
