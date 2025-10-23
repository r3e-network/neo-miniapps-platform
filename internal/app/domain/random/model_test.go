package random

import "testing"

func TestResultValueLength(t *testing.T) {
	res := Result{Value: []byte{1, 2, 3, 4}}
	if len(res.Value) != 4 {
		t.Fatalf("expected 4-byte result, got %d", len(res.Value))
	}
}
