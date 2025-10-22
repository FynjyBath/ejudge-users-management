package main

import "testing"

func TestNormalizeAuthorizationHeaderValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"already bearer", "Bearer AQAA123", "Bearer AQAA123"},
		{"lower bearer", "bearer token", "bearer token"},
		{"raw token", "AQAA123", "Bearer AQAA123"},
		{"bearer without space", "BearerAQAA123", "Bearer AQAA123"},
		{"token scheme", "Token foo", "Token foo"},
		{"custom scheme", "Custom value", "Custom value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAuthorizationHeaderValue(tt.input); got != tt.want {
				t.Fatalf("normalizeAuthorizationHeaderValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
