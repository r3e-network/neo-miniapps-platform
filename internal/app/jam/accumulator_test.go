package jam

import (
	"strings"
	"testing"
	"time"
)

func TestHashStrings(t *testing.T) {
	tests := []struct {
		name  string
		algo  string
		parts []string
	}{
		{
			name:  "blake3-256",
			algo:  "blake3-256",
			parts: []string{"hello", "world"},
		},
		{
			name:  "blake3",
			algo:  "blake3",
			parts: []string{"hello", "world"},
		},
		{
			name:  "sha256 default",
			algo:  "sha256",
			parts: []string{"hello", "world"},
		},
		{
			name:  "unknown algo uses sha256",
			algo:  "unknown",
			parts: []string{"hello", "world"},
		},
		{
			name:  "empty algo uses sha256",
			algo:  "",
			parts: []string{"test"},
		},
		{
			name:  "whitespace normalized",
			algo:  "  BLAKE3-256  ",
			parts: []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashStrings(tt.algo, tt.parts...)
			if result == "" {
				t.Error("expected non-empty hash")
			}
			// Should be hex encoded
			if len(result) != 64 { // 32 bytes = 64 hex chars for both blake3 and sha256
				t.Errorf("expected 64 char hash, got %d", len(result))
			}
			// Should be lowercase hex
			if result != strings.ToLower(result) {
				t.Error("hash should be lowercase")
			}
		})
	}
}

func TestHashStrings_Consistency(t *testing.T) {
	// Same input should produce same output
	h1 := hashStrings("blake3-256", "hello", "world")
	h2 := hashStrings("blake3-256", "hello", "world")
	if h1 != h2 {
		t.Errorf("inconsistent hash: %s != %s", h1, h2)
	}

	// Different input should produce different output
	h3 := hashStrings("blake3-256", "hello", "world!")
	if h1 == h3 {
		t.Error("different inputs should produce different hashes")
	}

	// Different order should produce different output
	h4 := hashStrings("blake3-256", "world", "hello")
	if h1 == h4 {
		t.Error("different order should produce different hash")
	}
}

func TestReportMetadataHash(t *testing.T) {
	report := WorkReport{
		PackageID:        "pkg-1",
		ServiceID:        "svc-1",
		RefineOutputHash: "hash123",
	}

	// Should produce consistent hash
	h1 := reportMetadataHash(report, "blake3-256")
	h2 := reportMetadataHash(report, "blake3-256")
	if h1 != h2 {
		t.Errorf("inconsistent report hash: %s != %s", h1, h2)
	}

	// Different report should produce different hash
	report2 := report
	report2.ServiceID = "svc-2"
	h3 := reportMetadataHash(report2, "blake3-256")
	if h1 == h3 {
		t.Error("different reports should produce different hashes")
	}

	// Different algo should produce different hash
	h4 := reportMetadataHash(report, "sha256")
	if h1 == h4 {
		t.Error("different algorithms should produce different hashes")
	}
}

func TestPackageMetadataHash(t *testing.T) {
	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{Kind: "test"}},
	}

	// Should produce consistent hash
	h1 := packageMetadataHash(pkg, "blake3-256")
	h2 := packageMetadataHash(pkg, "blake3-256")
	if h1 != h2 {
		t.Errorf("inconsistent package hash: %s != %s", h1, h2)
	}

	// Different package should produce different hash
	pkg2 := pkg
	pkg2.ServiceID = "svc-2"
	h3 := packageMetadataHash(pkg2, "blake3-256")
	if h1 == h3 {
		t.Error("different packages should produce different hashes")
	}
}

func TestDeriveRoot(t *testing.T) {
	now := time.Now().UTC()
	input := ReceiptInput{
		Hash:         "hash123",
		ServiceID:    "svc-1",
		EntryType:    ReceiptTypePackage,
		Status:       "applied",
		ProcessedAt:  now,
		MetadataHash: "meta123",
	}

	// Should produce valid root
	root, err := deriveRoot("prevRoot", input, 1, "blake3-256")
	if err != nil {
		t.Fatalf("deriveRoot failed: %v", err)
	}
	if root == "" {
		t.Error("expected non-empty root")
	}

	// Same inputs should produce same root
	root2, _ := deriveRoot("prevRoot", input, 1, "blake3-256")
	if root != root2 {
		t.Errorf("inconsistent root: %s != %s", root, root2)
	}

	// Different prevRoot should produce different root
	root3, _ := deriveRoot("differentPrev", input, 1, "blake3-256")
	if root == root3 {
		t.Error("different prevRoot should produce different root")
	}

	// Different seq should produce different root
	root4, _ := deriveRoot("prevRoot", input, 2, "blake3-256")
	if root == root4 {
		t.Error("different seq should produce different root")
	}

	// Different entry type should produce different root
	input2 := input
	input2.EntryType = ReceiptTypeReport
	root5, _ := deriveRoot("prevRoot", input2, 1, "blake3-256")
	if root == root5 {
		t.Error("different entry type should produce different root")
	}
}

func TestDeriveRoot_ZeroTime(t *testing.T) {
	input := ReceiptInput{
		Hash:         "hash123",
		ServiceID:    "svc-1",
		EntryType:    ReceiptTypePackage,
		Status:       "applied",
		ProcessedAt:  time.Time{}, // zero time
		MetadataHash: "meta123",
	}

	// Should not error with zero time (should use current time)
	root, err := deriveRoot("prevRoot", input, 1, "blake3-256")
	if err != nil {
		t.Fatalf("deriveRoot with zero time failed: %v", err)
	}
	if root == "" {
		t.Error("expected non-empty root")
	}
}

func TestReceiptType(t *testing.T) {
	// Test constants are defined
	if ReceiptTypePackage != "package" {
		t.Errorf("ReceiptTypePackage = %s, want package", ReceiptTypePackage)
	}
	if ReceiptTypeReport != "report" {
		t.Errorf("ReceiptTypeReport = %s, want report", ReceiptTypeReport)
	}
}
