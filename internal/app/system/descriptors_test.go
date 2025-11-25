package system

import (
	core "github.com/R3E-Network/service_layer/internal/services/core"
	"testing"
)

type mockProvider struct{ desc core.Descriptor }

func (m mockProvider) Descriptor() core.Descriptor { return m.desc }

func TestCollectDescriptors(t *testing.T) {
	providers := []DescriptorProvider{
		mockProvider{desc: core.Descriptor{Name: "svc1", Layer: core.LayerPlatform}},
		mockProvider{desc: core.Descriptor{Name: "svc2", Layer: core.LayerPlatform}},
		mockProvider{desc: core.Descriptor{Name: "svc3", Layer: core.LayerPlatform}},
		nil,
	}

	descr := CollectDescriptors(providers)

	if len(descr) != 3 {
		t.Fatalf("expected 3 descriptors, got %d", len(descr))
	}
	if descr[0].Name != "svc1" || descr[1].Name != "svc2" || descr[2].Name != "svc3" {
				 t.Fatalf("unexpected order: %#v", descr)
	}
}
