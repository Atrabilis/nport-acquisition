package passive

import (
	"encoding/binary"
	"testing"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

func TestDecodeLufftWSXCycle(t *testing.T) {
	frames := make([][]byte, 0, lufftWSXExpectedFrames)
	for i := 0; i < lufftWSXExpectedFrames; i++ {
		frames = append(frames, lufftWSXTestFrame(3, uint16(4166+i)))
	}

	values, err := decodeLufftWSXCycle(frames)
	if err != nil {
		t.Fatalf("decodeLufftWSXCycle returned error: %v", err)
	}
	if len(values) != lufftWSXExpectedFrames {
		t.Fatalf("len(values) = %d, want %d", len(values), lufftWSXExpectedFrames)
	}
	if values[0].Name != "value_0" || values[0].Type != "uint16" || values[0].Value != 4166 {
		t.Fatalf("values[0] = %#v, want value_0 uint16 4166", values[0])
	}
	if values[52].Name != "value_52" || values[52].Value != 4218 {
		t.Fatalf("values[52] = %#v, want value_52 4218", values[52])
	}
}

func TestFrameDecoderEmitsLufftWSXCycleWhenComplete(t *testing.T) {
	decoder := NewFrameDecoder(config.NPortConfig{
		DeviceType: "generic",
		Slaves: []config.SlaveConfig{
			{Name: "lufft_wsx_3", Address: 3, DeviceType: "lufft_wsx"},
		},
	})

	var values []RegisterValue
	var decoderLines []string
	for i := 0; i < lufftWSXExpectedFrames; i++ {
		frame := lufftWSXTestFrame(3, uint16(4166+i))
		summary := modbusrtu.Summarize(frame)
		_, _, _, values, decoderLines = decoder.DecodeFrame(summary, frame, frame[3:5])
		if i < lufftWSXExpectedFrames-1 && len(values) != 0 {
			t.Fatalf("unexpected values before next anchor at frame %d", i)
		}
	}
	if len(values) != lufftWSXExpectedFrames {
		t.Fatalf("len(values) = %d, want %d", len(values), lufftWSXExpectedFrames)
	}
	if len(decoderLines) == 0 {
		t.Fatal("expected decoder lines for completed cycle")
	}
}

func lufftWSXTestFrame(slaveID uint8, value uint16) []byte {
	frame := []byte{slaveID, 0x04, 0x02, 0x00, 0x00, 0x00, 0x00}
	binary.BigEndian.PutUint16(frame[3:5], value)
	crc := modbusrtu.CRC16(frame[:5])
	frame[5] = byte(crc)
	frame[6] = byte(crc >> 8)
	return frame
}
