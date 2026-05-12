package passive

import (
	"encoding/binary"
	"testing"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

func TestDecodeDustIQCycle(t *testing.T) {
	frames := make([][]byte, 0, dustIQExpectedFrames)
	for i := 0; i < dustIQExpectedFrames; i++ {
		frames = append(frames, dustIQTestFrame(1, uint16(800+i)))
	}

	values, warnings, err := decodeDustIQCycle(frames)
	if err != nil {
		t.Fatalf("decodeDustIQCycle returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(values) != 22 {
		t.Fatalf("len(values) = %d, want 22", len(values))
	}
	if values[0].Name != "ir_device_type" || values[0].Value != 800 {
		t.Fatalf("values[0] = %#v, want ir_device_type=800", values[0])
	}
	if values[6].Name != "ir_soiling_ratio_sensor1" || values[6].Value != 80.6 {
		t.Fatalf("values[6] = %#v, want ir_soiling_ratio_sensor1=80.6", values[6])
	}
}

func TestFrameDecoderEmitsDustIQCycleOnNextAnchor(t *testing.T) {
	decoder := NewFrameDecoder(config.NPortConfig{
		DeviceType: "generic",
		Slaves: []config.SlaveConfig{
			{Name: "dustiq_1", Address: 1, DeviceType: "dustiq"},
		},
	})

	var values []RegisterValue
	var decoderLines []string
	for i := 0; i < dustIQExpectedFrames; i++ {
		frame := dustIQTestFrame(1, uint16(800+i))
		summary := modbusrtu.Summarize(frame)
		_, _, _, values, decoderLines = decoder.DecodeFrame(summary, frame, frame[3:5])
		if len(values) != 0 {
			t.Fatalf("unexpected values before next anchor at frame %d", i)
		}
	}

	anchor := dustIQTestFrame(1, 800)
	summary := modbusrtu.Summarize(anchor)
	slaveName, deviceType, _, values, decoderLines := decoder.DecodeFrame(summary, anchor, anchor[3:5])
	if slaveName != "dustiq_1" {
		t.Fatalf("slaveName = %q, want dustiq_1", slaveName)
	}
	if deviceType != "dustiq" {
		t.Fatalf("deviceType = %q, want dustiq", deviceType)
	}
	if len(values) != 22 {
		t.Fatalf("len(values) = %d, want 22", len(values))
	}
	if len(decoderLines) == 0 {
		t.Fatal("expected decoder lines for completed cycle")
	}
}

func dustIQTestFrame(slaveID uint8, value uint16) []byte {
	frame := []byte{slaveID, 0x04, 0x02, 0x00, 0x00, 0x00, 0x00}
	binary.BigEndian.PutUint16(frame[3:5], value)
	crc := modbusrtu.CRC16(frame[:5])
	frame[5] = byte(crc)
	frame[6] = byte(crc >> 8)
	return frame
}
