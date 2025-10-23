package trigger

import "testing"

func TestTypeValues(t *testing.T) {
	if TypeCron != "cron" || TypeEvent != "event" || TypeWebhook != "webhook" {
		t.Fatalf("unexpected trigger type values")
	}
}

func TestTriggerConfig(t *testing.T) {
	trg := Trigger{Config: map[string]string{"timezone": "UTC"}}
	if trg.Config["timezone"] != "UTC" {
		t.Fatalf("expected config to persist")
	}
}
