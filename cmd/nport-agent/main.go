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
	if err := validateReadOnlyConfig(cfg); err != nil {
		fmt.Printf("invalid config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	duration := time.Duration(cfg.Agent.TestDurationSeconds) * time.Second
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				fmt.Printf("read-only duration reached (%s); shutting down\n", duration)
				stop()
			}
		}()
	}

	collector := passive.NewSlaveCollector()
	var wg sync.WaitGroup
	for _, nport := range cfg.NPorts {
		nport := nport
		if cfg.Agent.TestOnlyValidCRC {
			nport.SkipInvalidCRC = true
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			passive.Listen(ctx, nport, collector)
		}()
	}

	wg.Wait()
	if err := writeDiscoveryReport(*outputDir, *configPath, collector); err != nil {
		fmt.Printf("failed to write discovery report: %v\n", err)
	}
	fmt.Println("shutdown complete")
}

func validateReadOnlyConfig(cfg config.Config) error {
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
	case "store":
		return fmt.Errorf("sub_mode %q is not available in this first read-only port", cfg.Agent.SubMode)
	default:
		return fmt.Errorf("sub_mode %q not implemented", cfg.Agent.SubMode)
	}
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
