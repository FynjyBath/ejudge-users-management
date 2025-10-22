package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAuthorizationHeaderValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"already bearer", "Bearer AQAA123", "Bearer AQAA123"},
		{"lower bearer", "bearer token", "bearer token"},
		{"raw token", "AQAA123", "AQAA123"},
		{"bearer without space", "BearerAQAA123", "BearerAQAA123"},
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

func TestChangeRegistrationIncludesServerErrorMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(changeRegistrationPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": false, "error": {"message": "User is blocked", "num": 13, "symbol": "USER_BLOCKED", "log_id": "log-1"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := &http.Client{Timeout: time.Second}

	err := changeRegistration(client, server.URL, "token", 1, userSpec{Login: "user", Name: "User Name"}, actionRegister, time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "User is blocked") {
		t.Fatalf("error %q does not contain server message", err)
	}
}

func TestChangeRegistrationIncludesResultMessageWhenErrorMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(changeRegistrationPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": false, "result": "contest is closed"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := &http.Client{Timeout: time.Second}

	err := changeRegistration(client, server.URL, "token", 1, userSpec{Login: "user", Name: "User Name"}, actionRegister, time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "contest is closed") {
		t.Fatalf("error %q does not contain server message", err)
	}
}

func TestParseUsersNormalizesSemicolons(t *testing.T) {
	raw := "123:Alice; user2:User Two ;user3:User Three; \n"

	users, err := parseUsers(raw)
	if err != nil {
		t.Fatalf("parseUsers returned error: %v", err)
	}

	want := []userSpec{
		{ID: intPtr(123), Login: "123", Name: "Alice"},
		{Login: "user2", Name: "User Two"},
		{Login: "user3", Name: "User Three"},
	}

	if !reflect.DeepEqual(users, want) {
		t.Fatalf("parseUsers(%q) = %#v, want %#v", raw, users, want)
	}
}

func TestParseContestIDsSplitsByTab(t *testing.T) {
	raw := "101\t202\t 303\t"

	ids, err := parseContestIDs(raw)
	if err != nil {
		t.Fatalf("parseContestIDs returned error: %v", err)
	}

	want := []int{101, 202, 303}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("parseContestIDs(%q) = %#v, want %#v", raw, ids, want)
	}
}

func intPtr(v int) *int {
	return &v
}
