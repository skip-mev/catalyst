package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	logging "github.com/skip-mev/catalyst/internal/log"
	"golang.org/x/exp/slices"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/skip-mev/catalyst/internal/cosmos"
	cosmostypes "github.com/skip-mev/catalyst/internal/cosmos/types"
)

type LoadTestType string

const (
	LoadTestTypeEth    LoadTestType = "eth"
	LoadTestTypeCosmos LoadTestType = "cosmos"
)

var (
	loadTestTypes = []LoadTestType{LoadTestTypeCosmos, LoadTestTypeCosmos}
)

func main() {
	logger, _ := logging.DefaultLogger()
	defer logging.CloseLogFile()

	configPath := flag.String("config", "", "Path to load test configuration file")
	loadtestType := flag.String("type", "cosmos", "Load test type to use (cosmos, eth)")
	flag.Parse()

	if *configPath == "" {
		saveConfigError("config file path is required", logger)
		logger.Fatal("config file path is required")
	}

	testType := LoadTestType(*loadtestType)

	if !slices.Contains(loadTestTypes, testType) {
		saveConfigError("loadtest type must be one of: cosmos, eth", logger)
		logger.Fatal("loadtest type must be one of: cosmos, eth")
	}

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		saveConfigError("failed to read config file", logger)
		logger.Fatal("failed to read config file", zap.Error(err))
	}

	switch testType {
	case LoadTestTypeEth:
		panic("not supported yet")
	case LoadTestTypeCosmos:
		var spec cosmostypes.LoadTestSpec
		if err := yaml.Unmarshal(configData, &spec); err != nil {
			saveConfigError("failed to parse config file", logger)
			logger.Fatal("failed to parse config file", zap.Error(err))
		}

		ctx := context.Background()
		test, err := cosmos.New(ctx, spec)
		if err != nil {
			saveConfigError(fmt.Sprintf("failed to create test. error: %s", err), logger)
			logger.Fatal("failed to create test", zap.Error(err))
		}

		_, err = test.Run(ctx, logger)
		if err != nil {
			logger.Fatal("failed to run load test", zap.Error(err))
		}
	}

}

func saveConfigError(err string, logger *zap.Logger) {
	if saveErr := cosmos.SaveResults(cosmostypes.LoadTestResult{
		Error: err,
	}, logger); saveErr != nil {
		logger.Fatal("failed to save results", zap.Error(saveErr))
	}
}
