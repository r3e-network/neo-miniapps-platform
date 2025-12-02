package framework

import (
	"errors"
	"testing"
)

func TestComputeResult_Success(t *testing.T) {
	t.Run("successful result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: "data"}
		if !r.Success() {
			t.Error("Success() should return true")
		}
		if r.Failed() {
			t.Error("Failed() should return false")
		}
		if r.Error() != "" {
			t.Error("Error() should return empty string")
		}
	})

	t.Run("failed result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Err: errors.New("failed")}
		if r.Success() {
			t.Error("Success() should return false")
		}
		if !r.Failed() {
			t.Error("Failed() should return true")
		}
		if r.Error() != "failed" {
			t.Errorf("Error() = %q, want 'failed'", r.Error())
		}
	})
}

func TestComputeResult_ResultAs(t *testing.T) {
	t.Run("string result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: "hello"}
		var s string
		if err := r.ResultAs(&s); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if s != "hello" {
			t.Errorf("s = %q, want 'hello'", s)
		}
	})

	t.Run("int result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: 42}
		var i int
		if err := r.ResultAs(&i); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if i != 42 {
			t.Errorf("i = %d, want 42", i)
		}
	})

	t.Run("int64 from float64", func(t *testing.T) {
		// JSON unmarshaling produces float64 for numbers
		r := ComputeResult{Module: "test", Result: float64(100)}
		var i int64
		if err := r.ResultAs(&i); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if i != 100 {
			t.Errorf("i = %d, want 100", i)
		}
	})

	t.Run("float64 result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: 3.14}
		var f float64
		if err := r.ResultAs(&f); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if f != 3.14 {
			t.Errorf("f = %f, want 3.14", f)
		}
	})

	t.Run("bool result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: true}
		var b bool
		if err := r.ResultAs(&b); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if !b {
			t.Error("b should be true")
		}
	})

	t.Run("map result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: map[string]any{"key": "value"}}
		var m map[string]any
		if err := r.ResultAs(&m); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if m["key"] != "value" {
			t.Errorf("m[key] = %v, want 'value'", m["key"])
		}
	})

	t.Run("slice result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: []any{1, 2, 3}}
		var a []any
		if err := r.ResultAs(&a); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if len(a) != 3 {
			t.Errorf("len(a) = %d, want 3", len(a))
		}
	})

	t.Run("struct result via JSON", func(t *testing.T) {
		type Data struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		r := ComputeResult{Module: "test", Result: map[string]any{"name": "test", "value": float64(42)}}
		var d Data
		if err := r.ResultAs(&d); err != nil {
			t.Fatalf("ResultAs failed: %v", err)
		}
		if d.Name != "test" || d.Value != 42 {
			t.Errorf("d = %+v, want {Name:test Value:42}", d)
		}
	})

	t.Run("nil result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: nil}
		var s string
		if err := r.ResultAs(&s); err != nil {
			t.Fatalf("ResultAs with nil should not error: %v", err)
		}
	})

	t.Run("error result returns error", func(t *testing.T) {
		expectedErr := errors.New("compute failed")
		r := ComputeResult{Module: "test", Err: expectedErr}
		var s string
		err := r.ResultAs(&s)
		if err != expectedErr {
			t.Errorf("ResultAs error = %v, want %v", err, expectedErr)
		}
	})
}

func TestComputeResult_MustResultAs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: "hello"}
		var s string
		r.MustResultAs(&s)
		if s != "hello" {
			t.Errorf("s = %q, want 'hello'", s)
		}
	})

	t.Run("panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()

		r := ComputeResult{Module: "test", Err: errors.New("failed")}
		var s string
		r.MustResultAs(&s)
	})
}

func TestComputeResult_String(t *testing.T) {
	t.Run("successful result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Result: "data"}
		s := r.String()
		if s != `ComputeResult{Module: "test", Result: data}` {
			t.Errorf("String() = %q", s)
		}
	})

	t.Run("failed result", func(t *testing.T) {
		r := ComputeResult{Module: "test", Err: errors.New("failed")}
		s := r.String()
		if s != `ComputeResult{Module: "test", Error: "failed"}` {
			t.Errorf("String() = %q", s)
		}
	})
}

func TestComputeResults_AllSuccessful(t *testing.T) {
	t.Run("all successful", func(t *testing.T) {
		rs := ComputeResults{
			{Module: "a", Result: 1},
			{Module: "b", Result: 2},
		}
		if !rs.AllSuccessful() {
			t.Error("AllSuccessful() should return true")
		}
	})

	t.Run("one failed", func(t *testing.T) {
		rs := ComputeResults{
			{Module: "a", Result: 1},
			{Module: "b", Err: errors.New("failed")},
		}
		if rs.AllSuccessful() {
			t.Error("AllSuccessful() should return false")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		rs := ComputeResults{}
		if !rs.AllSuccessful() {
			t.Error("empty slice should return true")
		}
	})
}

func TestComputeResults_AnyFailed(t *testing.T) {
	t.Run("none failed", func(t *testing.T) {
		rs := ComputeResults{
			{Module: "a", Result: 1},
		}
		if rs.AnyFailed() {
			t.Error("AnyFailed() should return false")
		}
	})

	t.Run("one failed", func(t *testing.T) {
		rs := ComputeResults{
			{Module: "a", Result: 1},
			{Module: "b", Err: errors.New("failed")},
		}
		if !rs.AnyFailed() {
			t.Error("AnyFailed() should return true")
		}
	})
}

func TestComputeResults_Successful(t *testing.T) {
	rs := ComputeResults{
		{Module: "a", Result: 1},
		{Module: "b", Err: errors.New("failed")},
		{Module: "c", Result: 3},
	}

	successful := rs.Successful()
	if len(successful) != 2 {
		t.Errorf("Successful() len = %d, want 2", len(successful))
	}
	if successful[0].Module != "a" || successful[1].Module != "c" {
		t.Error("Successful() returned wrong results")
	}
}

func TestComputeResults_Failed(t *testing.T) {
	rs := ComputeResults{
		{Module: "a", Result: 1},
		{Module: "b", Err: errors.New("failed b")},
		{Module: "c", Err: errors.New("failed c")},
	}

	failed := rs.Failed()
	if len(failed) != 2 {
		t.Errorf("Failed() len = %d, want 2", len(failed))
	}
}

func TestComputeResults_ByModule(t *testing.T) {
	rs := ComputeResults{
		{Module: "a", Result: 1},
		{Module: "b", Result: 2},
	}

	t.Run("found", func(t *testing.T) {
		r := rs.ByModule("b")
		if r == nil {
			t.Fatal("ByModule should find module b")
		}
		if r.Result != 2 {
			t.Errorf("Result = %v, want 2", r.Result)
		}
	})

	t.Run("not found", func(t *testing.T) {
		r := rs.ByModule("c")
		if r != nil {
			t.Error("ByModule should return nil for unknown module")
		}
	})
}

func TestComputeResults_Modules(t *testing.T) {
	rs := ComputeResults{
		{Module: "a"},
		{Module: "b"},
		{Module: "c"},
	}

	modules := rs.Modules()
	if len(modules) != 3 {
		t.Errorf("Modules() len = %d, want 3", len(modules))
	}
	if modules[0] != "a" || modules[1] != "b" || modules[2] != "c" {
		t.Errorf("Modules() = %v", modules)
	}
}

func TestComputeResults_FirstError(t *testing.T) {
	t.Run("has error", func(t *testing.T) {
		expectedErr := errors.New("first error")
		rs := ComputeResults{
			{Module: "a", Result: 1},
			{Module: "b", Err: expectedErr},
			{Module: "c", Err: errors.New("second error")},
		}

		err := rs.FirstError()
		if err != expectedErr {
			t.Errorf("FirstError() = %v, want %v", err, expectedErr)
		}
	})

	t.Run("no error", func(t *testing.T) {
		rs := ComputeResults{
			{Module: "a", Result: 1},
		}

		err := rs.FirstError()
		if err != nil {
			t.Errorf("FirstError() = %v, want nil", err)
		}
	})
}

func TestComputeResults_Errors(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	rs := ComputeResults{
		{Module: "a", Result: 1},
		{Module: "b", Err: err1},
		{Module: "c", Err: err2},
	}

	errs := rs.Errors()
	if len(errs) != 2 {
		t.Errorf("Errors() len = %d, want 2", len(errs))
	}
	if errs[0] != err1 || errs[1] != err2 {
		t.Error("Errors() returned wrong errors")
	}
}

func TestComputeResults_Counts(t *testing.T) {
	rs := ComputeResults{
		{Module: "a", Result: 1},
		{Module: "b", Err: errors.New("failed")},
		{Module: "c", Result: 3},
	}

	if rs.Count() != 3 {
		t.Errorf("Count() = %d, want 3", rs.Count())
	}
	if rs.SuccessCount() != 2 {
		t.Errorf("SuccessCount() = %d, want 2", rs.SuccessCount())
	}
	if rs.FailedCount() != 1 {
		t.Errorf("FailedCount() = %d, want 1", rs.FailedCount())
	}
}

func TestNewComputeResult(t *testing.T) {
	r := NewComputeResult("test", "data")
	if r.Module != "test" {
		t.Errorf("Module = %q, want 'test'", r.Module)
	}
	if r.Result != "data" {
		t.Errorf("Result = %v, want 'data'", r.Result)
	}
	if r.Err != nil {
		t.Error("Err should be nil")
	}
}

func TestNewComputeResultError(t *testing.T) {
	expectedErr := errors.New("compute error")
	r := NewComputeResultError("test", expectedErr)
	if r.Module != "test" {
		t.Errorf("Module = %q, want 'test'", r.Module)
	}
	if r.Err != expectedErr {
		t.Errorf("Err = %v, want %v", r.Err, expectedErr)
	}
}
