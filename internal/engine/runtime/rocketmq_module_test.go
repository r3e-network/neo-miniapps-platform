package runtime

import (
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
)

func TestRocketMQModuleTopicFor(t *testing.T) {
	cfg := config.RocketMQConfig{
		TopicPrefix: "sl",
	}
	mod := newRocketMQModule(cfg)
	if got := mod.topicFor("My Event"); got != "sl.my-event" {
		t.Fatalf("expected default prefix with sanitized event, got %s", got)
	}

	cfg = config.RocketMQConfig{
		TopicPrefix: "svc",
		Namespace:   "dev",
	}
	mod = newRocketMQModule(cfg)
	if got := mod.topicFor("Hello.World"); got != "dev.svc.hello.world" {
		t.Fatalf("expected namespaced prefix, got %s", got)
	}
}
