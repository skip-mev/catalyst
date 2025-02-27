package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/skip-mev/catalyst/internal/loadtest"
	"github.com/skip-mev/catalyst/internal/types"
	"go.uber.org/zap"
	"os"
	"path/filepath"
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
	results, err := lt.runner.Run(ctx)
	if err != nil {
		results.Error = err.Error()
	}

	if saveErr := saveResults(results, logger); saveErr != nil {
		return results, fmt.Errorf("failed to save results: %w", saveErr)
	}

	lt.runner.GetCollector().PrintResults(results)

	return results, err
}

// todo: add timestamp suffix to file
// saveResults saves the load test results to /tmp/catalyst/load_test.json
func saveResults(results types.LoadTestResult, logger *zap.Logger) error {
	dir := "/tmp/catalyst"
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create results directory",
			zap.String("dir", dir),
			zap.Error(err))
		return err
	}

	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal results to JSON",
			zap.Error(err))
		return err
	}

	filePath := filepath.Join(dir, "load_test.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		logger.Error("Failed to write results to file",
			zap.String("path", filePath),
			zap.Error(err))
		return err
	}

	logger.Debug("Successfully saved load test results",
		zap.String("path", filePath))

	return nil
}
