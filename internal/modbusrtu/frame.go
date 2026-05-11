package modbusrtu

import (
	"fmt"
	"strings"
)

type Summary struct {
	Length       int
	SlaveID      uint8
	FunctionCode uint8
	DataLength   int
	ByteCount    *int
	CRC          uint16
	CRCValid     *bool
}

func Summarize(frame []byte) Summary {
	summary := Summary{Length: len(frame)}
	if len(frame) < 4 {
		return summary
	}

	summary.SlaveID = frame[0]
	summary.FunctionCode = frame[1]
	summary.DataLength = len(frame) - 4

	payload := frame[:len(frame)-2]
	expected := CRC16(payload)
	seen := uint16(frame[len(frame)-2]) | uint16(frame[len(frame)-1])<<8
	valid := expected == seen
	summary.CRC = seen
	summary.CRCValid = &valid

	if summary.DataLength > 0 {
		count := int(frame[2])
		summary.ByteCount = &count
	}
	return summary
}

func FormatSummary(summary Summary) string {
	if summary.Length == 0 {
		return "empty frame"
	}

	crcPart := "crc: n/a"
	if summary.CRCValid != nil {
		status := "bad"
		if *summary.CRCValid {
			status = "ok"
		}
		crcPart = fmt.Sprintf("crc: 0x%04X (%s)", summary.CRC, status)
	}

	byteCountPart := ""
	if summary.ByteCount != nil {
		byteCountPart = fmt.Sprintf(" byte_count=%d", *summary.ByteCount)
	}

	return fmt.Sprintf(
		"len=%d addr=%d func=0x%02X data_len=%d%s %s",
		summary.Length,
		summary.SlaveID,
		summary.FunctionCode,
		summary.DataLength,
		byteCountPart,
		crcPart,
	)
}

func CRC16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func Hex(data []byte) string {
	const table = "0123456789ABCDEF"
	out := make([]byte, 3*len(data))
	for i, b := range data {
		out[i*3] = table[b>>4]
		out[i*3+1] = table[b&0x0F]
		out[i*3+2] = ' '
	}
	if len(out) > 0 {
		out = out[:len(out)-1]
	}
	return string(out)
}

func DecimalBytes(data []byte) []int {
	out := make([]int, len(data))
	for i, b := range data {
		out[i] = int(b)
	}
	return out
}

func RegisterParserLines(data []byte) []string {
	if len(data)%2 != 0 {
		return nil
	}
	if len(data) == 2 {
		return []string{formatRegisterParsers(data)}
	}

	lines := make([]string, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		lines = append(lines, fmt.Sprintf("[%d] %s", i/2, formatRegisterParsers(data[i:i+2])))
	}
	return lines
}

func formatRegisterParsers(data []byte) string {
	if len(data) != 2 {
		return ""
	}
	u16be := uint16(data[0])<<8 | uint16(data[1])
	i16be := int16(u16be)
	return strings.Join([]string{
		fmt.Sprintf("u16be=%d", u16be),
		fmt.Sprintf("i16be=%d", i16be),
	}, ", ")
}
