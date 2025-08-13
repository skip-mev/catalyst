package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	cosmos "github.com/skip-mev/catalyst/chains/cosmos"
	eth "github.com/skip-mev/catalyst/chains/ethereum"
	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	logging "github.com/skip-mev/catalyst/chains/log"
	"github.com/skip-mev/catalyst/chains/types"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	cosmostypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

var (
	validKinds = []string{eth.Kind, cosmos.Kind}
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

	// register chain-specific subconfigs so the shared spec can decode chain_config.
	cosmostypes.Register()
	ethtypes.Register()

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

	switch kind {
	case eth.Kind:
		test, err := eth.New(ctx, logger, spec)
		if err != nil {
			saveConfigError(fmt.Sprintf("failed to create eth test. error: %s", err), logger)
			logger.Fatal("failed to create eth test", zap.Error(err))
		}
		if _, err = test.Run(ctx, logger); err != nil {
			logger.Fatal("failed to run eth load test", zap.Error(err))
		}

	case cosmos.Kind:
		test, err := cosmos.New(ctx, spec)
		if err != nil {
			saveConfigError(fmt.Sprintf("failed to create cosmos test. error: %s", err), logger)
			logger.Fatal("failed to create cosmos test", zap.Error(err))
		}
		if _, err = test.Run(ctx, logger); err != nil {
			logger.Fatal("failed to run cosmos load test", zap.Error(err))
		}

	default:
		saveConfigError(fmt.Sprintf("invalid kind: must be one of %s", strings.Join(validKinds, ",")), logger)
		logger.Fatal(fmt.Sprintf("invalid kind: must be one of %s", strings.Join(validKinds, ",")))
	}
}

func saveConfigError(err string, logger *zap.Logger) {
	if saveErr := types.SaveResults(types.LoadTestResult{
		Error: err,
	}, logger); saveErr != nil {
		logger.Fatal("failed to save results", zap.Error(saveErr))
	}
}
