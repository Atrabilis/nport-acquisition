package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

const TimescaleShadowType = "timescaledb_shadow"

var nonIdentChars = regexp.MustCompile(`[^a-z0-9_]+`)
var multiUnderscore = regexp.MustCompile(`_+`)

type TimescaleShadowWriter struct {
	name         string
	fqn          string
	deviceTypes  map[string]struct{}
	slaveNames   map[string]struct{}
	slaveIDs     map[uint8]struct{}
	hasAnyFilter bool
	pool         *pgxpool.Pool
}

type ShadowRow struct {
	Plant      string
	TS         time.Time
	DeviceName string
	SlaveName  string
	SlaveID    uint8
	Fields     map[string]interface{}
	Tags       map[string]string
}

func NewTimescaleShadowWriter(ctx context.Context, name string, cfg config.TimescaledbShadowConfig) (*TimescaleShadowWriter, error) {
	host := strings.TrimSpace(os.Getenv(cfg.HostEnv))
	port := strings.TrimSpace(os.Getenv(cfg.PortEnv))
	user := strings.TrimSpace(os.Getenv(cfg.UserEnv))
	password := os.Getenv(cfg.PasswordEnv)
	database := strings.TrimSpace(os.Getenv(cfg.DatabaseEnv))
	if host == "" || port == "" || user == "" || password == "" || database == "" {
		return nil, fmt.Errorf("timescale env is incomplete for output %q", name)
	}

	schema := sanitizeIdentifier(cfg.Schema)
	table := sanitizeIdentifier(cfg.Table)
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		url.QueryEscape(user),
		url.QueryEscape(password),
		host,
		port,
		url.QueryEscape(database),
	)

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("create timescale pool for output %q: %w", name, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping timescale output %q: %w", name, err)
	}

	return &TimescaleShadowWriter{
		name:         name,
		fqn:          schema + "." + table,
		deviceTypes:  normalizedStringSet(cfg.DeviceTypes),
		slaveNames:   normalizedStringSet(cfg.SlaveNames),
		slaveIDs:     uint8Set(cfg.SlaveIDs),
		hasAnyFilter: len(cfg.DeviceTypes) > 0 || len(cfg.SlaveNames) > 0 || len(cfg.SlaveIDs) > 0,
		pool:         pool,
	}, nil
}

func (w *TimescaleShadowWriter) Name() string {
	if w == nil {
		return ""
	}
	return w.name
}

func (w *TimescaleShadowWriter) Close() {
	if w != nil && w.pool != nil {
		w.pool.Close()
	}
}

func (w *TimescaleShadowWriter) Accepts(row ShadowRow) bool {
	if w == nil || !w.hasAnyFilter {
		return true
	}
	if len(w.deviceTypes) > 0 {
		deviceType := strings.TrimSpace(row.Tags["device_type"])
		if _, ok := w.deviceTypes[normalizeFilterToken(deviceType)]; !ok {
			return false
		}
	}
	if len(w.slaveNames) > 0 {
		if _, ok := w.slaveNames[normalizeFilterToken(row.SlaveName)]; !ok {
			return false
		}
	}
	if len(w.slaveIDs) > 0 {
		if _, ok := w.slaveIDs[row.SlaveID]; !ok {
			return false
		}
	}
	return true
}

func (w *TimescaleShadowWriter) Write(ctx context.Context, row ShadowRow) error {
	if w == nil || w.pool == nil {
		return nil
	}
	if row.Plant == "" || row.DeviceName == "" || row.SlaveName == "" {
		return fmt.Errorf("plant/device_name/slave_name are required")
	}
	if len(row.Fields) == 0 {
		return nil
	}

	tags := map[string]string{}
	for k, v := range row.Tags {
		tags[k] = v
	}
	tags["plant"] = row.Plant
	tags["device_name"] = row.DeviceName
	tags["slave_name"] = row.SlaveName
	tags["slave_id"] = strconv.Itoa(int(row.SlaveID))

	seriesKey, flags := buildSeriesMetadata(tags)
	payload := map[string]interface{}{
		"slave_id":   int(row.SlaveID),
		"series_key": seriesKey,
		"flags":      flags,
		"fields":     row.Fields,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	ts := row.TS.UTC()
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (plant, ts, device_name, slave_name, series_key, payload) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (plant, device_name, slave_name, ts, series_key) DO UPDATE SET payload = EXCLUDED.payload, ingested_at = now()",
		w.fqn,
	)
	_, err = w.pool.Exec(ctx, query, row.Plant, ts, row.DeviceName, row.SlaveName, seriesKey, json.RawMessage(payloadJSON))
	if err != nil {
		return fmt.Errorf("upsert %s: %w", w.fqn, err)
	}
	return nil
}

func normalizedStringSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeFilterToken(value)
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func uint8Set(values []uint8) map[uint8]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[uint8]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func normalizeFilterToken(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = multiUnderscore.ReplaceAllString(normalized, "_")
	return strings.Trim(normalized, "_")
}

func buildSeriesMetadata(tags map[string]string) (string, map[string]string) {
	flags := map[string]string{}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := strings.TrimSpace(tags[k])
		if v == "" || shouldSkipSeriesTagKey(k) {
			continue
		}
		flags[k] = v
	}
	if len(flags) == 0 {
		return "default", flags
	}

	canonicalKeys := make([]string, 0, len(flags))
	for k := range flags {
		canonicalKeys = append(canonicalKeys, k)
	}
	sort.Strings(canonicalKeys)

	parts := make([]string, 0, len(canonicalKeys))
	for _, k := range canonicalKeys {
		parts = append(parts, strings.ToLower(strings.TrimSpace(k))+"="+strings.TrimSpace(flags[k]))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "h1:" + hex.EncodeToString(sum[:]), flags
}

func shouldSkipSeriesTagKey(tagKey string) bool {
	switch strings.ToLower(strings.TrimSpace(tagKey)) {
	case "ts", "plant", "device", "device_name", "slave", "slave_name", "slave_id", "unit":
		return true
	default:
		return false
	}
}

func sanitizeIdentifier(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.ReplaceAll(v, "-", "_")
	v = strings.ReplaceAll(v, " ", "_")
	v = nonIdentChars.ReplaceAllString(v, "_")
	v = multiUnderscore.ReplaceAllString(v, "_")
	v = strings.Trim(v, "_")
	if v == "" {
		v = "field"
	}
	if v[0] >= '0' && v[0] <= '9' {
		v = "f_" + v
	}
	return v
}
