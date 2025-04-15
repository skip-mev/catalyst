package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/skip-mev/catalyst/pkg/types"
	"os"
	"path/filepath"

	"github.com/skip-mev/catalyst/internal/loadtest"
	"go.uber.org/zap"
)

// LoadTest represents a load test that can be executed
type LoadTest struct {
	runner *loadtest.Runner
}

// New creates a new load test from a specification
func New(ctx context.Context, spec types.LoadTestSpec) (*LoadTest, error) {
	runner, err := loadtest.NewRunner(ctx, spec)
	if err != nil {
		return nil, err
	}

	return &LoadTest{
		runner: runner,
	}, nil
}

// Run executes the load test and returns the results
func (lt *LoadTest) Run(ctx context.Context, logger *zap.Logger) (types.LoadTestResult, error) {
	logger.Info("starting new load test run")
	results, err := lt.runner.Run(ctx)
	if err != nil {
		results.Error = err.Error()
	}
	logger.Info("runner results", zap.Any("results", results))

	lt.runner.GetCollector().PrintResults(results)

	logger.Info("load test run completed, saving results")

	if saveErr := SaveResults(results, logger); saveErr != nil {
		return results, fmt.Errorf("failed to save results: %w", saveErr)
	}

	return results, err
}

// todo: add timestamp suffix to file
// saveResults saves the load test results to /catalyst/load_test.json
func SaveResults(results types.LoadTestResult, logger *zap.Logger) error {
	dir := "/tmp/catalyst"
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("failed to create results directory",
			zap.String("dir", dir),
			zap.Error(err))
		return err
	}

	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		logger.Error("failed to marshal results to JSON",
			zap.Error(err))
		return err
	}

	filePath := filepath.Join(dir, "load_test.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		logger.Error("failed to write results to file",
			zap.String("path", filePath),
			zap.Error(err))
		return err
	}

	logger.Debug("successfully saved load test results",
		zap.String("path", filePath))

	return nil
}
