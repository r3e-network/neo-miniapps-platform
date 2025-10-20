package system

import (
	"context"
	"errors"
	"testing"
)

type mockService struct {
	name       string
	startCount int
	stopCount  int
	startErr   error
}

func (m *mockService) Name() string { return m.name }

func (m *mockService) Start(context.Context) error {
	m.startCount++
	return m.startErr
}

func (m *mockService) Stop(context.Context) error {
	m.stopCount++
	return nil
}

func TestManagerStartStopOrder(t *testing.T) {
	mgr := NewManager()
	services := []*mockService{{name: "a"}, {name: "b"}, {name: "c"}}
	for _, svc := range services {
		if err := mgr.Register(svc); err != nil {
			t.Fatalf("register %s: %v", svc.name, err)
		}
	}

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	if err := mgr.Stop(context.Background()); err != nil {
		t.Fatalf("stop manager: %v", err)
	}

	for _, svc := range services {
		if svc.startCount != 1 {
			t.Fatalf("service %s expected start once", svc.name)
		}
		if svc.stopCount != 1 {
			t.Fatalf("service %s expected stop once", svc.name)
		}
	}
}

func TestManagerRollbackOnStartFailure(t *testing.T) {
	mgr := NewManager()
	good := &mockService{name: "good"}
	bad := &mockService{name: "bad", startErr: errors.New("boom")}

	if err := mgr.Register(good); err != nil {
		t.Fatalf("register good: %v", err)
	}
	if err := mgr.Register(bad); err != nil {
		t.Fatalf("register bad: %v", err)
	}

	if err := mgr.Start(context.Background()); err == nil {
		t.Fatalf("expected start error")
	}

	if good.stopCount == 0 {
		t.Fatalf("expected good service to be stopped after failure")
	}
}
