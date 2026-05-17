package main

import "testing"

func TestResolveCredInput(t *testing.T) {
	cases := []struct {
		name             string
		kind, reg, user  string
		wantKind, wantUs string
		wantErr          bool
	}{
		{"registry ok", "registry", "ghcr.io", "octocat", "registry", "octocat", false},
		{"registry case-insensitive", "Registry", "ghcr.io", "u", "registry", "u", false},
		{"registry missing url", "registry", "", "u", "", "", true},
		{"registry missing user", "registry", "ghcr.io", "", "", "", true},
		{"git defaults username", "git", "", "", "git", "x-access-token", false},
		{"git keeps explicit username", "git", "", "deploy-bot", "git", "deploy-bot", false},
		{"git ignores registry", "git", "ghcr.io", "", "git", "x-access-token", false},
		{"unknown kind", "ssh", "", "", "", "", true},
		{"empty kind", "", "", "", "", "", true},
		{"whitespace trimmed", "  git  ", "", "", "git", "x-access-token", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, u, err := resolveCredInput(tc.kind, tc.reg, tc.user)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got kind=%q user=%q", k, u)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k != tc.wantKind || u != tc.wantUs {
				t.Errorf("got (kind=%q,user=%q), want (kind=%q,user=%q)", k, u, tc.wantKind, tc.wantUs)
			}
		})
	}
}

func TestNormalizeSecret(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"trailing LF trimmed", "tok\n", "tok", false},
		{"trailing CRLF trimmed", "tok\r\n", "tok", false},
		{"no trailing newline", "tok", "tok", false},
		{"internal newline kept", "line1\nline2\n", "line1\nline2", false},
		{"only newline is empty", "\n", "", true},
		{"empty is error", "", "", true},
		{"whitespace-only is not empty", "  ", "  ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeSecret(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("normalizeSecret(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
