package devpack

import "testing"

func TestActionTypes(t *testing.T) {
	a := EnsureGasAccount(map[string]interface{}{"wallet": "w1"})
	if a.Type != ActionGasbankEnsureAccount {
		t.Fatalf("action type mismatch: %s", a.Type)
	}
	b := GenerateRandom(map[string]interface{}{"length": 16})
	if b.Type != ActionRandomGenerate {
		t.Fatalf("action type mismatch: %s", b.Type)
	}
	df := SubmitDataFeedUpdate(map[string]interface{}{"feedId": "f1", "roundId": 1, "price": "1"})
	if df.Type != ActionDatafeedSubmitUpdate {
		t.Fatalf("action type mismatch: %s", df.Type)
	}
	ds := PublishDataStreamFrame(map[string]interface{}{"streamId": "s1", "sequence": 1})
	if ds.Type != ActionDatastreamPublishFrame {
		t.Fatalf("action type mismatch: %s", ds.Type)
	}
	ref := AsResult(b, map[string]interface{}{"label": "rand"})
	if !ref.Ref || ref.Type != ActionRandomGenerate {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestResponseHelpers(t *testing.T) {
	ok := Success(map[string]string{"msg": "hi"}, nil)
	if !ok.Success || ok.Data == nil {
		t.Fatalf("expected success response")
	}
	fail := Failure("boom", map[string]string{"code": "err"})
	if fail.Success || fail.Error == nil {
		t.Fatalf("expected failure response")
	}
}
