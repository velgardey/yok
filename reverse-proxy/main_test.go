package main

import (
	"testing"
	"time"
)

func TestHostLabel(t *testing.T) {
	tests := []struct {
		host     string
		site     string
		expected string
		wantErr  bool
	}{
		{host: "bold-otter-run.yok.ninja", site: "yok.ninja", expected: "bold-otter-run"},
		{host: "bold-otter-run.yok.ninja:443", site: "yok.ninja", expected: "bold-otter-run"},
		{host: "BOLD-otter-run.YOK.NINJA.", site: "yok.ninja", expected: "bold-otter-run"},
		{host: "01d7f245-ed3a-4153-9305-2e1f52b40256.yok.ninja", site: "yok.ninja", expected: "01d7f245-ed3a-4153-9305-2e1f52b40256"},
		{host: "evil.example.com", site: "yok.ninja", wantErr: true},
		{host: "yok.ninja", site: "yok.ninja", wantErr: true},
		{host: "a.b.yok.ninja", site: "yok.ninja", wantErr: true},
	}
	for _, tt := range tests {
		label, err := hostLabel(tt.host, tt.site)
		if tt.wantErr {
			if err == nil {
				t.Errorf("hostLabel(%q, %q): expected error, got label %q", tt.host, tt.site, label)
			}
			continue
		}
		if err != nil {
			t.Errorf("hostLabel(%q, %q): unexpected error %v", tt.host, tt.site, err)
		} else if label != tt.expected {
			t.Errorf("hostLabel(%q, %q) = %q, want %q", tt.host, tt.site, label, tt.expected)
		}
	}
}

func TestStaticPath(t *testing.T) {
	tests := []struct{ in, out string }{
		{"/", "/index.html"},
		{"", "/index.html"},
		{"/docs/", "/docs/index.html"},
		{"/assets/app.js", "/assets/app.js"},
		{"/logo.svg", "/logo.svg"},
		{"/about/team", "/index.html"}, // SPA client-side routing
		{"/about", "/index.html"},
	}
	for _, tt := range tests {
		if got := staticPath(tt.in); got != tt.out {
			t.Errorf("staticPath(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestResolveCache(t *testing.T) {
	c := newResolveCache(time.Minute, time.Minute)

	if _, ok := c.get("missing"); ok {
		t.Fatal("expected cache miss for unknown slug")
	}

	c.put("known", "dep-id")
	if id, ok := c.get("known"); !ok || id != "dep-id" {
		t.Fatalf("expected hit for known slug, got id=%q ok=%v", id, ok)
	}

	c.put("gone", "")
	if _, ok := c.get("gone"); !ok {
		t.Fatal("expected negative entry to be cached")
	}

	c.put("expired", "x")
	c.entries["expired"] = cacheEntry{deploymentID: "x", expiresAt: time.Now().Add(-time.Second)}
	if _, ok := c.get("expired"); ok {
		t.Fatal("expected expired entry to be evicted")
	}
}
