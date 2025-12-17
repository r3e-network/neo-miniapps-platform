package contract

import (
	"encoding/json"
	"os"
	"testing"
)

type neoManifest struct {
	ABI struct {
		Methods []neoMethod `json:"methods"`
	} `json:"abi"`
}

type neoMethod struct {
	Name       string     `json:"name"`
	Parameters []neoParam `json:"parameters"`
}

type neoParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func TestContractABIAlignsWithEdgePayloads(t *testing.T) {
	SkipIfNoCompiledContracts(t)

	loadManifest := func(contractName string) neoManifest {
		t.Helper()

		_, manifestPath, err := FindContractArtifacts(contractName)
		if err != nil {
			t.Fatalf("FindContractArtifacts(%s): %v", contractName, err)
		}

		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read manifest %s: %v", manifestPath, err)
		}

		var m neoManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("parse manifest %s: %v", manifestPath, err)
		}
		return m
	}

	expectParams := func(contractName, method string, wantTypes []string) {
		t.Helper()

		m := loadManifest(contractName)

		var found *neoMethod
		for i := range m.ABI.Methods {
			if m.ABI.Methods[i].Name == method {
				found = &m.ABI.Methods[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("%s manifest missing method %q", contractName, method)
		}

		if len(found.Parameters) != len(wantTypes) {
			t.Fatalf("%s.%s params=%d, want %d", contractName, method, len(found.Parameters), len(wantTypes))
		}

		for i := range wantTypes {
			if found.Parameters[i].Type != wantTypes[i] {
				t.Fatalf("%s.%s param[%d] type=%s, want %s", contractName, method, i, found.Parameters[i].Type, wantTypes[i])
			}
		}
	}

	// Edge contract invocations use String IDs (app_id, proposal_id, request_id).
	expectParams("PaymentHub", "pay", []string{"String", "Integer", "String"})
	expectParams("Governance", "vote", []string{"String", "Boolean", "Integer"})
	expectParams("Governance", "createProposal", []string{"String", "String", "Integer", "Integer"})
	expectParams("RandomnessLog", "record", []string{"String", "ByteArray", "ByteArray", "Integer"})
	expectParams("RandomnessLog", "get", []string{"String"})
	expectParams("AppRegistry", "register", []string{"String", "ByteArray", "ByteArray", "ByteArray"})
	expectParams("AppRegistry", "getApp", []string{"String"})
	expectParams("AppRegistry", "setStatus", []string{"String", "Integer"})
}

