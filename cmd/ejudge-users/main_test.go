package main

import (
	"strings"
	"testing"
)

func TestDecodeChangeRegistrationResponseHTMLWithEmbeddedJSON(t *testing.T) {
	body := []byte(`<!DOCTYPE html><html><body><style>body{margin:0;}</style><pre>{"ok":true,"result":true,"action":"upsert"}</pre></body></html>`)
	reply, err := decodeChangeRegistrationResponse(body, "text/html; charset=utf-8", "200 OK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reply.OK || !reply.Result {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if reply.Action != "upsert" {
		t.Fatalf("unexpected action: %q", reply.Action)
	}
}

func TestDecodeChangeRegistrationResponseInvalidJSON(t *testing.T) {
	_, err := decodeChangeRegistrationResponse([]byte("not json"), "application/json", "200 OK")
	if err == nil || !strings.Contains(err.Error(), "decoding response") {
		t.Fatalf("expected decoding error, got %v", err)
	}
}

func TestDecodeChangeRegistrationResponseHTMLWithoutJSON(t *testing.T) {
	_, err := decodeChangeRegistrationResponse([]byte("<html><body>No JSON here</body></html>"), "text/html", "200 OK")
	if err == nil || !strings.Contains(err.Error(), "unexpected response content type") {
		t.Fatalf("expected content type error, got %v", err)
	}
}

func TestExtractJSONFromTextSkipsNonJSONFragments(t *testing.T) {
	fragment, ok := extractJSONFromText([]byte("<style>.x{color:red;}</style><script>var data = {invalid:true};</script><pre>{\"ok\":true}</pre>"))
	if !ok {
		t.Fatalf("expected to find embedded JSON")
	}
	if string(fragment) != "{\"ok\":true}" {
		t.Fatalf("unexpected fragment: %s", fragment)
	}
}
