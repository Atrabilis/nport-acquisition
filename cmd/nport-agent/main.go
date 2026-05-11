package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/passive"
	"github.com/Atrabilis/nport-acquisition/internal/storage"
	"github.com/Atrabilis/nport-acquisition/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yml", "path to config file")
	outputDir := flag.String("output-dir", "test", "directory for read-only discovery reports")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}
	if err := validateConfig(cfg); err != nil {
		fmt.Printf("invalid config: %v\n", err)
		os.Exit(1)
	}
	storeMode := isStoreMode(cfg.Agent.SubMode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	duration := runDuration(cfg)
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				fmt.Printf("run duration reached (%s); shutting down\n", duration)
				stop()
			}
		}()
	}

	collector := passive.NewSlaveCollector()
	var recorder passive.FrameRecorder
	if storeMode {
		writers, err := buildWriters(ctx, cfg)
		if err != nil {
			fmt.Printf("failed to initialize storage: %v\n", err)
			os.Exit(1)
		}
		coordinator := store.NewCoordinator(cfg, writers, stop)
		if coordinator == nil {
			fmt.Println("failed to initialize storage: no writers or detected_slaves configured")
			os.Exit(1)
		}
		defer coordinator.Close()
		recorder = coordinator
	}

	var wg sync.WaitGroup
	for _, nport := range cfg.NPorts {
		nport := nport
		if cfg.Agent.TestOnlyValidCRC {
			nport.SkipInvalidCRC = true
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			passive.Listen(ctx, nport, collector, recorder)
		}()
	}

	wg.Wait()
	if err := writeDiscoveryReport(*outputDir, *configPath, collector); err != nil {
		fmt.Printf("failed to write discovery report: %v\n", err)
	}
	fmt.Println("shutdown complete")
}

func validateConfig(cfg config.Config) error {
	if len(cfg.NPorts) == 0 {
		return fmt.Errorf("config contains no nports")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Agent.Mode))
	if mode == "" {
		mode = "passive-listening"
	}
	if mode != "passive-listening" {
		return fmt.Errorf("mode %q not implemented", cfg.Agent.Mode)
	}

	subMode := strings.ToLower(strings.TrimSpace(cfg.Agent.SubMode))
	if subMode == "" {
		subMode = "read-only"
	}
	switch subMode {
	case "read-only", "readonly", "test":
		return nil
	case "store", "write":
		if len(cfg.Storage.Outputs) == 0 {
			return fmt.Errorf("sub_mode %q requires storage.outputs", cfg.Agent.SubMode)
		}
		return nil
	default:
		return fmt.Errorf("sub_mode %q not implemented", cfg.Agent.SubMode)
	}
}

func isStoreMode(subMode string) bool {
	switch strings.ToLower(strings.TrimSpace(subMode)) {
	case "store", "write":
		return true
	default:
		return false
	}
}

func runDuration(cfg config.Config) time.Duration {
	seconds := cfg.Agent.TestDurationSeconds
	if isStoreMode(cfg.Agent.SubMode) && cfg.Agent.StoreTimeoutSeconds > 0 {
		seconds = cfg.Agent.StoreTimeoutSeconds
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func buildWriters(ctx context.Context, cfg config.Config) ([]store.Writer, error) {
	var writers []store.Writer
	for _, output := range cfg.Storage.Outputs {
		if !output.IsEnabled() {
			continue
		}
		outputType := strings.ToLower(strings.TrimSpace(output.Type))
		switch outputType {
		case storage.TimescaleShadowType:
			writer, err := storage.NewTimescaleShadowWriter(ctx, output.Name, output.TimescaledbShadow)
			if err != nil {
				return nil, err
			}
			fmt.Printf("[store] storage output %s (%s) ready\n", output.Name, outputType)
			writers = append(writers, writer)
		default:
			return nil, fmt.Errorf("storage output %q type %q not implemented", output.Name, output.Type)
		}
	}
	if len(writers) == 0 {
		return nil, fmt.Errorf("no enabled storage outputs")
	}
	return writers, nil
}

func writeDiscoveryReport(outputDir string, configPath string, collector *passive.SlaveCollector) error {
	if collector == nil {
		return nil
	}
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	base := strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
	if base == "" {
		base = "config"
	}
	outPath := filepath.Join(outputDir, "slave_ids_detected_"+base+".txt")
	if err := collector.WriteFile(outPath); err != nil {
		return err
	}
	fmt.Println(outPath + " written")
	return nil
}
