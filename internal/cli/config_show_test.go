package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "(not set)"},
		{"ab", "**"},
		{"abcd", "****"},
		{"abcde", "*bcde"},
		{"secretkey1234", "*********1234"},
	}
	for _, tc := range cases {
		got := maskAPIKey(tc.input)
		if got != tc.want {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestConfigShow(t *testing.T) {
	t.Setenv("HB_HATENA_ID", "myuser")
	t.Setenv("HB_BLOG_ID", "myuser.hateblo.jp")
	t.Setenv("HB_API_KEY", "secretkey1234")

	cmd := newConfigShowCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "hatena_id: myuser") {
		t.Errorf("expected hatena_id in output, got: %s", got)
	}
	if !strings.Contains(got, "blog_id:   myuser.hateblo.jp") {
		t.Errorf("expected blog_id in output, got: %s", got)
	}
	// API key should be masked
	if strings.Contains(got, "secretkey1234") {
		t.Errorf("API key should be masked, got: %s", got)
	}
	if !strings.Contains(got, "*********1234") {
		t.Errorf("expected masked API key in output, got: %s", got)
	}
	if !strings.Contains(got, "concurrency: 5 (default)") {
		t.Errorf("expected default concurrency in output, got: %s", got)
	}
	if !strings.Contains(got, "max_pages: unlimited") {
		t.Errorf("expected unlimited max_pages in output, got: %s", got)
	}
}
