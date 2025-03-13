package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	logging "github.com/skip-mev/catalyst/internal/shared"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/loadtest"
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

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		saveConfigError("failed to read config file", logger)
		logger.Fatal("failed to read config file", zap.Error(err))
	}

	var spec loadtesttypes.LoadTestSpec
	if err := yaml.Unmarshal(configData, &spec); err != nil {
		saveConfigError("failed to parse config file", logger)
		logger.Fatal("failed to parse config file", zap.Error(err))
	}

	ctx := context.Background()
	test, err := loadtest.New(ctx, spec)
	if err != nil {
		saveConfigError(fmt.Sprintf("failed to create test. error: %s", err), logger)
		logger.Fatal("failed to create test", zap.Error(err))
	}

	_, err = test.Run(ctx, logger)
	if err != nil {
		logger.Fatal("failed to run load test", zap.Error(err))
	}
}

func saveConfigError(err string, logger *zap.Logger) {
	if saveErr := loadtest.SaveResults(loadtesttypes.LoadTestResult{
		Error: err,
	}, logger); saveErr != nil {
		logger.Fatal("failed to save results", zap.Error(saveErr))
	}
}
