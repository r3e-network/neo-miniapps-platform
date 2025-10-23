package random

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestServiceGenerate(t *testing.T) {
	svc := New(nil)
	res, err := svc.Generate(context.Background(), 32)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(res.Value) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(res.Value))
	}
	if res.CreatedAt.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
	zero := make([]byte, 32)
	if string(res.Value) == string(zero) {
		t.Fatalf("random bytes should not be all zero")
	}

	encoded := EncodeResult(res)
	if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
		t.Fatalf("encoded result not valid base64: %v", err)
	}
}

func TestServiceGenerateInvalidLength(t *testing.T) {
	svc := New(nil)
	invalidLengths := []int{-1, 0, 2048}
	for _, length := range invalidLengths {
		if _, err := svc.Generate(context.Background(), length); err == nil {
			t.Fatalf("expected error for length %d", length)
		}
	}
}

func ExampleService_Generate() {
	svc := New(nil)
	res, _ := svc.Generate(context.Background(), 4)
	fmt.Printf("bytes:%d encoded:%d\n", len(res.Value), len(EncodeResult(res)))
	// Output:
	// bytes:4 encoded:8
}
