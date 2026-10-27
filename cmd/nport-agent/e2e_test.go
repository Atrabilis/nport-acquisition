package main

// End-to-end tests: build the real binary and run it in store mode against a
// fake NPort, a TCP server that replays Modbus RTU bus traffic (master
// requests, slave responses, noise), and a real PostgreSQL shadow table.
//
// They need a PostgreSQL server and are skipped unless NPORT_E2E_PG is set to
// a connection URL, e.g. postgres://postgres:postgres@127.0.0.1:5432/postgres
// (CI provides one as a service container). If NPORT_E2E_COVERDIR is set, the
// binary is built with -cover and writes its coverage data there.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
	"github.com/jackc/pgx/v5"
)

// Same table layout the ansible-infra playbook ensure_timescale_shadow_table.yml creates.
const shadowTableDDL = `CREATE TABLE %s (
  plant TEXT NOT NULL,
  ts TIMESTAMPTZ NOT NULL,
  device_name TEXT NOT NULL,
  slave_name TEXT NOT NULL,
  payload JSONB NOT NULL,
  series_key TEXT NOT NULL,
  ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (plant, device_name, slave_name, ts, series_key)
)`

func withCRC(frame []byte) []byte {
	crc := modbusrtu.CRC16(frame)
	return append(frame, byte(crc), byte(crc>>8))
}

// readRequest is what a Modbus master sends to read 43 holding registers.
func readRequest(slave byte) []byte {
	return withCRC([]byte{slave, 0x03, 0x00, 0x00, 0x00, 43})
}

// kippResponse answers readRequest with register i = base + i.
func kippResponse(slave byte, base uint16) []byte {
	frame := []byte{slave, 0x03, 2 * 43}
	for i := uint16(0); i < 43; i++ {
		v := base + i
		frame = append(frame, byte(v>>8), byte(v))
	}
	return withCRC(frame)
}

type busChunk struct {
	data  []byte
	pause time.Duration // silence on the bus after this chunk
}

// startFakeNPort serves the given bus traffic in a loop to every connection,
// like an NPort in TCP server mode relaying a serial bus.
func startFakeNPort(t *testing.T, script []busChunk) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				for {
					for _, chunk := range script {
						if _, err := c.Write(chunk.data); err != nil {
							return
						}
						time.Sleep(chunk.pause)
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// busTraffic is one polling cycle of a master reading two Kipp & Zonen
// sensors: line noise, a request and its response with a normal silence, and
// a second response sent right after another one with no silence in between,
// so the agent receives both in one chunk and has to split them.
func busTraffic() []busChunk {
	gap := 60 * time.Millisecond
	return []busChunk{
		{[]byte{0x00, 0xFF, 0x13}, gap},
		{readRequest(1), gap},
		{kippResponse(1, 1000), gap},
		{readRequest(2), gap},
		{append(kippResponse(1, 1000), kippResponse(2, 2000)...), gap},
	}
}

type nportE2E struct {
	conn   *pgx.Conn
	bin    string
	schema string
	env    []string
}

func setupNPortE2E(t *testing.T) *nportE2E {
	t.Helper()
	dsn := os.Getenv("NPORT_E2E_PG")
	if dsn == "" {
		t.Skip("NPORT_E2E_PG not set; skipping end-to-end test")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse NPORT_E2E_PG: %v", err)
	}
	password, _ := u.User.Password()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })

	schema := fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	if _, err := conn.Exec(ctx, fmt.Sprintf(shadowTableDDL, schema+".nport_kipp_zonen_shadow")); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "nport-agent")
	args := []string{"build", "-o", bin}
	if os.Getenv("NPORT_E2E_COVERDIR") != "" {
		args = append(args, "-cover", "-coverpkg=github.com/Atrabilis/nport-acquisition/...")
	}
	if out, err := exec.Command("go", append(args, ".")...).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	env := append(os.Environ(),
		"E2E_PG_HOST="+u.Hostname(),
		"E2E_PG_PORT="+u.Port(),
		"E2E_PG_USER="+u.User.Username(),
		"E2E_PG_PASSWORD="+password,
		"E2E_PG_DB="+strings.TrimPrefix(u.Path, "/"),
	)
	if dir := os.Getenv("NPORT_E2E_COVERDIR"); dir != "" {
		env = append(env, "GOCOVERDIR="+dir)
	}
	return &nportE2E{conn: conn, bin: bin, schema: schema, env: env}
}

// config follows chepica_np24_port02_piras.yml with a local port and short
// timings.
func (e *nportE2E) config(t *testing.T, port int, detected []int, timeoutSeconds int) string {
	t.Helper()
	ids := make([]string, len(detected))
	for i, id := range detected {
		ids[i] = fmt.Sprint(id)
	}
	cfg := fmt.Sprintf(`site: e2e_plant
agent:
  mode: passive-listening
  sub_mode: store
  store_timeout_seconds: %d
nports:
  - name: np_port02_piras
    host: 127.0.0.1
    port: %d
    device_type: kipp_zonen
    idle_gap_ms: 20
    serial: {baud: 19200, data_bits: 8, parity: even, stop_bits: 1}
    dial_timeout_ms: 2000
    reconnect_delay_ms: 200
    detected_slaves: [%s]
    slaves:
      - {name: kipp_zonen_1, address: 1}
      - {name: kipp_zonen_2, address: 2}
      - {name: kipp_zonen_3, address: 3}
storage:
  outputs:
    - name: local_kipp_zonen_shadow
      type: timescaledb_shadow
      enabled: true
      timescaledb_shadow:
        host_env: E2E_PG_HOST
        port_env: E2E_PG_PORT
        user_env: E2E_PG_USER
        password_env: E2E_PG_PASSWORD
        database_env: E2E_PG_DB
        schema: %s
        table: nport_kipp_zonen_shadow
        device_types: [kipp_zonen]
`, timeoutSeconds, port, strings.Join(ids, ", "), e.schema)
	path := filepath.Join(t.TempDir(), "np_port02_piras.yml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (e *nportE2E) run(t *testing.T, configPath, reportDir string) (string, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.bin, "-config", configPath, "-output-dir", reportDir)
	cmd.Env = e.env
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nport-agent failed: %v\n%s", err, out)
	}
	return string(out), time.Since(start)
}

func TestE2EStoresOneRowPerExpectedSlave(t *testing.T) {
	e := setupNPortE2E(t)
	port := startFakeNPort(t, busTraffic())
	reportDir := t.TempDir()

	out, elapsed := e.run(t, e.config(t, port, []int{1, 2}, 20), reportDir)
	// The run ends as soon as every expected slave was captured, well before
	// store_timeout_seconds.
	if elapsed > 10*time.Second {
		t.Fatalf("expected the run to stop once both slaves were captured, took %s\n%s", elapsed, out)
	}

	rows, err := e.conn.Query(context.Background(), "SELECT plant, device_name, slave_name, payload FROM "+e.schema+".nport_kipp_zonen_shadow ORDER BY slave_name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		plant, device, slave string
		slaveID              float64
		fields               map[string]float64
	}
	var got []row
	for rows.Next() {
		var r row
		var payload []byte
		if err := rows.Scan(&r.plant, &r.device, &r.slave, &payload); err != nil {
			t.Fatal(err)
		}
		var p struct {
			SlaveID float64            `json:"slave_id"`
			Fields  map[string]float64 `json:"fields"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			t.Fatalf("payload %s: %v", payload, err)
		}
		r.slaveID, r.fields = p.SlaveID, p.Fields
		got = append(got, r)
	}
	if len(got) != 2 {
		t.Fatalf("expected one row per expected slave (2), got %d\n%s", len(got), out)
	}
	for i, base := range []float64{1000, 2000} {
		r := got[i]
		wantSlave := fmt.Sprintf("kipp_zonen_%d", i+1)
		if r.plant != "e2e_plant" || r.device != "np_port02_piras" || r.slave != wantSlave || r.slaveID != float64(i+1) {
			t.Fatalf("row %d: unexpected identity %+v", i, r)
		}
		if len(r.fields) != 43 {
			t.Fatalf("%s: expected 43 registers, got %d", wantSlave, len(r.fields))
		}
		for reg := 0; reg < 43; reg++ {
			if v := r.fields[fmt.Sprintf("value_%d", reg)]; v != base+float64(reg) {
				t.Fatalf("%s value_%d = %v, want %v", wantSlave, reg, v, base+float64(reg))
			}
		}
	}

	report, err := os.ReadFile(filepath.Join(reportDir, "slave_ids_detected_np_port02_piras.txt"))
	if err != nil {
		t.Fatalf("discovery report: %v", err)
	}
	for _, id := range []string{"1", "2"} {
		if !strings.Contains(string(report), id) {
			t.Fatalf("discovery report should list slave %s:\n%s", id, report)
		}
	}
}

// When store_timeout_seconds runs out before every expected slave was seen,
// the captured slaves are still written and the missing ones are logged.
// Slave 3 never answers here; slaves 1 and 2 must not be lost because of it.
func TestE2EIncompleteCaptureWritesCapturedSlaves(t *testing.T) {
	e := setupNPortE2E(t)
	port := startFakeNPort(t, busTraffic())

	out, elapsed := e.run(t, e.config(t, port, []int{1, 2, 3}, 3), t.TempDir())
	if elapsed < 3*time.Second {
		t.Fatalf("expected the run to wait for store_timeout_seconds, ended after %s\n%s", elapsed, out)
	}
	if !strings.Contains(out, "missing: np_port02_piras/3") {
		t.Fatalf("the missing slave should be logged\n%s", out)
	}
	rows, err := e.conn.Query(context.Background(), "SELECT slave_name FROM "+e.schema+".nport_kipp_zonen_shadow ORDER BY slave_name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if strings.Join(got, ",") != "kipp_zonen_1,kipp_zonen_2" {
		t.Fatalf("expected the two captured slaves to be written, got %v\n%s", got, out)
	}
}
