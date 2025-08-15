package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/skip-mev/catalyst/chains"
	logging "github.com/skip-mev/catalyst/chains/log"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func main() {
	logger, _ := logging.DefaultLogger()
	defer logging.CloseLogFile()

	configPath := flag.String("config", "", "Path to load test configuration file")
	flag.Parse()

	if *configPath == "" {
		saveConfigError("config file path is required", logger)
		logger.Fatal("config file path is required")
	}

	data, err := os.ReadFile(*configPath)
	if err != nil {
		saveConfigError("failed to read config file", logger)
		logger.Fatal("failed to read config file", zap.Error(err))
	}

	// unmarshal into the single shared spec (with custom UnmarshalYAML).
	var spec loadtesttypes.LoadTestSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		saveConfigError("failed to parse config file", logger)
		logger.Fatal("failed to parse config file", zap.Error(err))
	}

	if err := spec.Validate(); err != nil {
		saveConfigError("failed to validate config file", logger)
		logger.Fatal("failed to validate config file", zap.Error(err))
	}

	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	if kind == "" {
		saveConfigError("config is missing required field 'kind'", logger)
		logger.Fatal("config is missing required field 'kind'")
	}

	ctx := context.Background()

	test, err := chains.NewLoadTest(ctx, logger, spec)
	if err != nil {
		saveConfigError(fmt.Sprintf("failed to create %s test. error: %s", kind, err), logger)
		logger.Fatal("failed to create load test", zap.Error(err))
	}

	if _, err = test.Run(ctx, logger); err != nil {
		logger.Fatal("failed to run load test", zap.Error(err))
	}
}

func saveConfigError(err string, logger *zap.Logger) {
	if saveErr := chains.SaveResults(loadtesttypes.LoadTestResult{
		Error: err,
	}, logger); saveErr != nil {
		logger.Fatal("failed to save results", zap.Error(saveErr))
	}
}
