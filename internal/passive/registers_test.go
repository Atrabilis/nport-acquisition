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

	slaveName, deviceType, lines, values := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "kipp_zonen",
		Slaves: []config.SlaveConfig{
			{Name: "piranometro_4", Address: 4},
		},
	}, 4, data)

	if slaveName != "piranometro_4" {
		t.Fatalf("slaveName = %q, want piranometro_4", slaveName)
	}
	if deviceType != "kipp_zonen" {
		t.Fatalf("deviceType = %q, want kipp_zonen", deviceType)
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
	if values[42].Type != "uint16" {
		t.Fatalf("values[42].Type = %q, want uint16", values[42].Type)
	}
}

func TestDecodeKnownRegistersAllowsKippZonnenAlias(t *testing.T) {
	data := make([]byte, 86)

	_, _, lines, _ := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "kipp_zonnen",
		Slaves: []config.SlaveConfig{
			{Name: "legacy_name", Address: 1},
		},
	}, 1, data)

	if len(lines) != 43 {
		t.Fatalf("len(lines) = %d, want 43", len(lines))
	}
}

func TestDecodeKnownRegistersUsesSlaveDeviceType(t *testing.T) {
	data := make([]byte, 86)

	slaveName, deviceType, lines, values := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "generic",
		Slaves: []config.SlaveConfig{
			{Name: "unknown_1", Address: 1, DeviceType: "generic"},
			{Name: "piranometro_2", Address: 2, DeviceType: "kipp_zonen"},
		},
	}, 2, data)

	if slaveName != "piranometro_2" {
		t.Fatalf("slaveName = %q, want piranometro_2", slaveName)
	}
	if deviceType != "kipp_zonen" {
		t.Fatalf("deviceType = %q, want kipp_zonen", deviceType)
	}
	if len(lines) != 43 {
		t.Fatalf("len(lines) = %d, want 43", len(lines))
	}
	if len(values) != 43 {
		t.Fatalf("len(values) = %d, want 43", len(values))
	}
}

func TestDecodeKnownRegistersSkipsGenericSlaveWithoutRegisters(t *testing.T) {
	data := make([]byte, 2)

	slaveName, deviceType, lines, values := decodeKnownRegisters(config.NPortConfig{
		DeviceType: "generic",
		Slaves: []config.SlaveConfig{
			{Name: "unknown_1", Address: 1, DeviceType: "generic"},
		},
	}, 1, data)

	if slaveName != "" {
		t.Fatalf("slaveName = %q, want empty", slaveName)
	}
	if deviceType != "generic" {
		t.Fatalf("deviceType = %q, want generic", deviceType)
	}
	if len(lines) != 0 {
		t.Fatalf("len(lines) = %d, want 0", len(lines))
	}
	if len(values) != 0 {
		t.Fatalf("len(values) = %d, want 0", len(values))
	}
}
