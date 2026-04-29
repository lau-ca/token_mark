package controller

import (
	"strings"
	"testing"
)

func TestParseCodexRTImportEntriesFromRawLines(t *testing.T) {
	entries, err := parseCodexRTImportEntries(codexRTImportRequest{
		RefreshTokens: []string{" rt-from-array "},
		Credentials: `
# comment
rt-one
rt-two
`,
	})
	if err != nil {
		t.Fatalf("parse entries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{"rt-from-array", "rt-one", "rt-two"}
	for i, entry := range entries {
		if entry.RefreshToken != want[i] {
			t.Fatalf("entry %d refresh_token = %q, want %q", i, entry.RefreshToken, want[i])
		}
	}
}

func TestParseCodexRTImportEntriesFromJSONObject(t *testing.T) {
	entries, err := parseCodexRTImportEntries(codexRTImportRequest{
		Credentials: `{"refreshToken":"rt-json","channelName":"Codex Pro","email":"user@example.com","accountId":"acc_123"}`,
	})
	if err != nil {
		t.Fatalf("parse entries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.RefreshToken != "rt-json" || entry.Name != "Codex Pro" || entry.Email != "user@example.com" || entry.AccountID != "acc_123" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestParseCodexRTImportEntriesFromJSONArray(t *testing.T) {
	entries, err := parseCodexRTImportEntries(codexRTImportRequest{
		Credentials: `[
			"rt-string",
			{"refresh_token":"rt-object","name":"Codex Team","chatgpt_account_id":"acc_456"}
		]`,
	})
	if err != nil {
		t.Fatalf("parse entries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].RefreshToken != "rt-string" {
		t.Fatalf("entry 0 refresh_token = %q", entries[0].RefreshToken)
	}
	if entries[1].RefreshToken != "rt-object" || entries[1].Name != "Codex Team" || entries[1].AccountID != "acc_456" {
		t.Fatalf("unexpected entry 1: %+v", entries[1])
	}
}

func TestNormalizeCodexRTImportCommaList(t *testing.T) {
	got := normalizeCodexRTImportCommaList(" default, vip,default, ", []string{"fallback"})
	if got != "default,vip" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeCodexRTImportCommaList("", []string{"a", "a", "b"}); got != "a,b" {
		t.Fatalf("fallback got %q", got)
	}
}

func TestBuildCodexRTImportChannelName(t *testing.T) {
	if got := buildCodexRTImportChannelName("Codex", "", "user@example.com", "acc_1234567890"); got != "Codex user@example.com" {
		t.Fatalf("got %q", got)
	}
	got := buildCodexRTImportChannelName("Codex", "", "", "acc_1234567890")
	if !strings.HasSuffix(got, "34567890") {
		t.Fatalf("got %q", got)
	}
}
