package passive

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

func Listen(ctx context.Context, nport config.NPortConfig, collector *SlaveCollector) {
	addr := net.JoinHostPort(nport.Host, strconv.Itoa(nport.Port))
	idleGap := deriveIdleGap(nport, 5*time.Millisecond)
	reconnectDelay := durationOrDefault(nport.ReconnectDelayMS, 2*time.Second)
	dialTimeout := durationOrDefault(nport.DialTimeoutMS, 2*time.Second)
	readBufSize := nport.ReadBufferBytes
	if readBufSize <= 0 {
		readBufSize = 1024
	}
	maxFrame := nport.MaxFrameBytes
	if maxFrame <= 0 {
		maxFrame = 4096
	}

	for {
		if ctx.Err() != nil {
			return
		}

		conn, err := net.DialTimeout("tcp", addr, dialTimeout)
		if err != nil {
			fmt.Printf("[%s] dial failed: %v (retrying in %s)\n", nport.Name, err, reconnectDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnectDelay):
				continue
			}
		}

		fmt.Printf("[%s] connected to %s\n", nport.Name, addr)
		if err := streamFrames(ctx, conn, nport, idleGap, readBufSize, maxFrame, collector); err != nil {
			fmt.Printf("[%s] connection closed: %v\n", nport.Name, err)
		}
		_ = conn.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

func streamFrames(ctx context.Context, conn net.Conn, nport config.NPortConfig, idleGap time.Duration, readBufSize int, maxFrame int, collector *SlaveCollector) error {
	buf := make([]byte, readBufSize)
	var frame []byte

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		_ = conn.SetReadDeadline(time.Now().Add(idleGap))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if len(frame) > 0 {
					logFrame(nport, frame, collector)
					frame = frame[:0]
				} else if nport.ConnectionKeepLog {
					fmt.Printf("[%s] idle\n", nport.Name)
				}
				continue
			}
			if errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrDeadlineExceeded) {
				return err
			}
			return fmt.Errorf("read error: %w", err)
		}

		frame = append(frame, buf[:n]...)
		if len(frame) >= maxFrame {
			logFrame(nport, frame, collector)
			frame = frame[:0]
		}
	}
}

func logFrame(nport config.NPortConfig, frame []byte, collector *SlaveCollector) {
	if len(frame) == 0 {
		return
	}

	summary := modbusrtu.Summarize(frame)
	if nport.SkipInvalidCRC && (summary.CRCValid == nil || !*summary.CRCValid) {
		return
	}
	if collector != nil && summary.CRCValid != nil && *summary.CRCValid {
		collector.Record(nport.Name, summary.SlaveID)
	}

	var dataDec string
	var parserLines []string
	var registerLines []string
	if summary.CRCValid != nil && *summary.CRCValid && len(frame) > 4 {
		start := 2
		if summary.ByteCount != nil {
			start = 3
		}
		if start < len(frame)-2 {
			data := frame[start : len(frame)-2]
			dataDec = fmt.Sprintf("%v", modbusrtu.DecimalBytes(data))
			if summary.ByteCount != nil && *summary.ByteCount == len(data) && len(data)%2 == 0 {
				parserLines = modbusrtu.RegisterParserLines(data)
				_, registerLines, _ = decodeKnownRegisters(nport, summary.SlaveID, data)
			}
		}
	}

	header := fmt.Sprintf("[%s] frame: %s", nport.Name, modbusrtu.FormatSummary(summary))
	if nport.LogFrameHex {
		header += " | hex: " + modbusrtu.Hex(frame)
	}

	lines := []string{header}
	if dataDec != "" {
		lines = append(lines, "  data_dec: "+dataDec)
	}
	if len(parserLines) > 0 {
		lines = append(lines, "  parsers:")
		for _, parserLine := range parserLines {
			lines = append(lines, "    "+parserLine)
		}
	}
	if len(registerLines) > 0 {
		lines = append(lines, "  registers:")
		for _, registerLine := range registerLines {
			lines = append(lines, "    "+registerLine)
		}
	}
	fmt.Println(strings.Join(lines, "\n"))
}

func durationOrDefault(ms int, fallback time.Duration) time.Duration {
	if ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return fallback
}

func deriveIdleGap(nport config.NPortConfig, fallback time.Duration) time.Duration {
	if nport.IdleGapMS > 0 {
		return time.Duration(nport.IdleGapMS) * time.Millisecond
	}
	baud := nport.Serial.Baud
	if baud <= 0 {
		return fallback
	}
	dataBits := nport.Serial.DataBits
	if dataBits <= 0 {
		dataBits = 8
	}
	stopBits := nport.Serial.StopBits
	if stopBits <= 0 {
		stopBits = 1
	}
	parityBits := 0.0
	switch strings.ToLower(nport.Serial.Parity) {
	case "even", "odd", "mark", "space":
		parityBits = 1
	}
	bitsPerChar := 1.0 + float64(dataBits) + parityBits + stopBits
	idle := (bitsPerChar / float64(baud)) * 3.5
	duration := time.Duration(idle * float64(time.Second))
	if duration <= 0 {
		return fallback
	}
	return duration
}
