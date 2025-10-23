package version

import (
    "strings"
    "testing"
)

func TestFullVersionContainsFields(t *testing.T) {
    Version = "1.2.3"
    GitCommit = "abcdef"
    BuildTime = "now"

    fv := FullVersion()
    if fv == "" || !containsAll(fv, []string{"1.2.3", "abcdef", "now"}) {
        t.Fatalf("full version missing details: %s", fv)
    }

    if ua := UserAgent(); ua != "ServiceLayer/1.2.3" {
        t.Fatalf("unexpected user agent %s", ua)
    }
}

func containsAll(s string, parts []string) bool {
    for _, part := range parts {
        if !strings.Contains(s, part) {
            return false
        }
    }
    return true
}
