package main

import (
	"context"
	"flag"
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/loadtest"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	configPath := flag.String("config", "", "Path to load test configuration file")
	flag.Parse()

	if *configPath == "" {
		logger.Fatal("Config file path is required")
	}

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		logger.Fatal("Failed to read config file", zap.Error(err))
	}

	var spec loadtesttypes.LoadTestSpec
	if err := yaml.Unmarshal(configData, &spec); err != nil {
		logger.Fatal("Failed to parse config file", zap.Error(err))
	}

	logger.Info("load test spec", zap.Any("spec", spec))

	ctx := context.Background()
	test, err := loadtest.New(ctx, spec)
	if err != nil {
		logger.Fatal("Failed to create test", zap.Error(err))
	}

	_, err = test.Run(ctx, logger)
	if err != nil {
		logger.Fatal("Failed to run load test", zap.Error(err))
	}
}
