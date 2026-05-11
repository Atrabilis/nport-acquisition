package modbusrtu

import "testing"

func TestCRC16KnownModbusFrame(t *testing.T) {
	frameWithoutCRC := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	if got, want := CRC16(frameWithoutCRC), uint16(0xCDC5); got != want {
		t.Fatalf("CRC16() = 0x%04X, want 0x%04X", got, want)
	}
}

func TestSummarizeValidFrame(t *testing.T) {
	frame := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0xC5, 0xCD}
	summary := Summarize(frame)

	if summary.Length != len(frame) {
		t.Fatalf("Length = %d, want %d", summary.Length, len(frame))
	}
	if summary.SlaveID != 1 {
		t.Fatalf("SlaveID = %d, want 1", summary.SlaveID)
	}
	if summary.FunctionCode != 0x03 {
		t.Fatalf("FunctionCode = 0x%02X, want 0x03", summary.FunctionCode)
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		t.Fatalf("CRCValid = %v, want true", summary.CRCValid)
	}
}
