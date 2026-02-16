package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

func TestNormalizeBasicCredential(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already encoded",
			input: "YWRtaW46YWRtaW4=",
			want:  "YWRtaW46YWRtaW4=",
		},
		{
			name:  "username password",
			input: "admin:admin",
			want:  "YWRtaW46YWRtaW4=",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeBasicCredential(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeBasicCredential(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveAPIBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		apiPath string
		want    string
	}{
		{
			name:    "default path",
			host:    "bam.example.internal",
			apiPath: "",
			want:    "https://bam.example.internal/api/v2",
		},
		{
			name:    "custom path",
			host:    "https://bam.example.internal",
			apiPath: "/api/v2",
			want:    "https://bam.example.internal/api/v2",
		},
		{
			name:    "host already has path",
			host:    "https://bam.example.internal/api/v2",
			apiPath: "",
			want:    "https://bam.example.internal/api/v2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveAPIBaseURL(tc.host, tc.apiPath)
			if err != nil {
				t.Fatalf("resolveAPIBaseURL(%q, %q) returned error: %v", tc.host, tc.apiPath, err)
			}
			if got != tc.want {
				t.Fatalf("resolveAPIBaseURL(%q, %q) = %q, want %q", tc.host, tc.apiPath, got, tc.want)
			}
		})
	}
}

func TestToRelativeName(t *testing.T) {
	tests := []struct {
		name string
		fqdn string
		zone string
		want string
	}{
		{
			name: "record under zone",
			fqdn: "_acme-challenge.example.com.",
			zone: "example.com.",
			want: "_acme-challenge",
		},
		{
			name: "zone apex",
			fqdn: "example.com.",
			zone: "example.com.",
			want: "@",
		},
		{
			name: "zone mismatch",
			fqdn: "_acme-challenge.other.com.",
			zone: "example.com.",
			want: "_acme-challenge.other.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toRelativeName(tc.fqdn, tc.zone)
			if got != tc.want {
				t.Fatalf("toRelativeName(%q, %q) = %q, want %q", tc.fqdn, tc.zone, got, tc.want)
			}
		})
	}
}

func TestFindTXTRecordMatches(t *testing.T) {
	records := []map[string]any{
		{
			"id":           10,
			"type":         "TXTRecord",
			"absoluteName": "_acme-challenge.example.com",
			"rdata":        "\"token-1\"",
		},
		{
			"id":           11,
			"type":         "TXTRecord",
			"absoluteName": "_acme-challenge.example.com",
			"rdata":        "\"token-2\"",
		},
	}

	matches := findTXTRecordMatches(records, "_acme-challenge.example.com.", "_acme-challenge", "token-1")
	if len(matches) != 1 {
		t.Fatalf("expected 1 matching record, got %d", len(matches))
	}
}

func TestValidateConfig(t *testing.T) {
	valid := customDNSProviderConfig{
		APIHost:               "https://bam.example.internal",
		Zone:                  "example.com",
		Username:              "cert-manager",
		PasswordSecretRef:     secretRef("bluecat-auth", "password"),
		InsecureSkipTLSVerify: true,
	}

	if err := validateConfig(valid); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestIsQuickDeployEnabled(t *testing.T) {
	cfgDefault := customDNSProviderConfig{}
	if !isQuickDeployEnabled(cfgDefault) {
		t.Fatalf("expected quickDeploy default to true")
	}

	disabled := false
	cfgDisabled := customDNSProviderConfig{QuickDeploy: &disabled}
	if isQuickDeployEnabled(cfgDisabled) {
		t.Fatalf("expected quickDeploy=false to disable quick deploy")
	}

	enabled := true
	cfgEnabled := customDNSProviderConfig{QuickDeploy: &enabled}
	if !isQuickDeployEnabled(cfgEnabled) {
		t.Fatalf("expected quickDeploy=true to enable quick deploy")
	}
}

func TestTriggerQuickDeploy(t *testing.T) {
	const zoneID int64 = 12345

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/zones/12345/deployments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/hal+json" {
			t.Fatalf("unexpected Accept header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/hal+json" {
			t.Fatalf("unexpected Content-Type header: %q", got)
		}
		if got := r.Header.Get("x-bcn-force-zone-retransfer"); got != "false" {
			t.Fatalf("unexpected x-bcn-force-zone-retransfer header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("invalid JSON payload: %v", err)
		}
		if payload["type"] != "QuickDeployment" {
			t.Fatalf("unexpected deployment type: %v", payload["type"])
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := &bluecatClient{
		baseURL:    server.URL + "/api/v2",
		httpClient: server.Client(),
		authHeader: "Bearer fake-token",
	}

	if err := client.triggerQuickDeploy(context.Background(), zoneID); err != nil {
		t.Fatalf("triggerQuickDeploy returned error: %v", err)
	}
}

func TestTriggerQuickDeployRequiresZoneID(t *testing.T) {
	client := &bluecatClient{}
	if err := client.triggerQuickDeploy(context.Background(), 0); err == nil {
		t.Fatalf("expected error when zone ID is missing")
	}
}

func secretRef(name, key string) *cmmeta.SecretKeySelector {
	return &cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{
			Name: name,
		},
		Key: key,
	}
}
