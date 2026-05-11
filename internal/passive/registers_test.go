package passive

import (
	"testing"

	"github.com/Atrabilis/nport-acquisition/internal/config"
)

func TestDecodeKnownRegistersUsesKippZonenProfile(t *testing.T) {
	data := make([]byte, 86)
	for i := 0; i < 43; i++ {
		data[i*2+1] = byte(i)
	}

	slaveName, lines, values := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "kipp_zonen",
		Slaves: []config.SlaveConfig{
			{Name: "piranometro_4", Address: 4},
		},
	}, 4, data)

	if slaveName != "piranometro_4" {
		t.Fatalf("slaveName = %q, want piranometro_4", slaveName)
	}
	if len(lines) != 43 {
		t.Fatalf("len(lines) = %d, want 43", len(lines))
	}
	if len(values) != 43 {
		t.Fatalf("len(values) = %d, want 43", len(values))
	}
	if values[42].Register != 42 || values[42].Name != "value42" || values[42].Value != 42 {
		t.Fatalf("values[42] = %#v, want register 42/value42/42", values[42])
	}
}

func TestDecodeKnownRegistersAllowsKippZonnenAlias(t *testing.T) {
	data := make([]byte, 86)

	_, lines, _ := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "kipp_zonnen",
		Slaves: []config.SlaveConfig{
			{Name: "legacy_name", Address: 1},
		},
	}, 1, data)

	if len(lines) != 43 {
		t.Fatalf("len(lines) = %d, want 43", len(lines))
	}
}
