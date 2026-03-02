package lsp

import "testing"

func TestClientCloseIsIdempotent(t *testing.T) {
	client := &Client{}

	if err := client.Close(); err != nil {
		t.Fatalf("first close returned error: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("second close returned error: %v", err)
	}
}
