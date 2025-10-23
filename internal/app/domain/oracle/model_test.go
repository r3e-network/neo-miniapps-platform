package oracle

import "testing"

func TestRequestStatusStrings(t *testing.T) {
	expected := map[RequestStatus]string{
		StatusPending:   "pending",
		StatusRunning:   "running",
		StatusSucceeded: "succeeded",
		StatusFailed:    "failed",
	}
	for status, value := range expected {
		if string(status) != value {
			t.Fatalf("expected %q, got %q", value, status)
		}
	}
}

func TestDataSourceHeaders(t *testing.T) {
	src := DataSource{Headers: map[string]string{"X-Key": "value"}}
	if src.Headers["X-Key"] != "value" {
		t.Fatalf("expected header to persist")
	}
}
