package chains

import (
	"context"
	"fmt"

	cosmosrunner "github.com/skip-mev/catalyst/chains/cosmos/runner"
	ethrunner "github.com/skip-mev/catalyst/chains/ethereum/runner"
	"github.com/skip-mev/catalyst/types"
	"go.uber.org/zap"
)

const (
	EthKind    = "eth"
	CosmosKind = "cosmos"
)

// Runner defines the interface that all chain-specific runners must implement
type Runner interface {
	Run(ctx context.Context) (types.LoadTestResult, error)
	PrintResults(result types.LoadTestResult)
}

// LoadTest represents a unified load test that can be executed for any chain kind
type LoadTest struct {
	runner Runner
	kind   string
}

// NewLoadTest creates a new load test from a specification
func NewLoadTest(ctx context.Context, logger *zap.Logger, spec types.LoadTestSpec) (*LoadTest, error) {
	var runner Runner

	switch spec.Kind {
	case EthKind:
		ethRunner, runnerErr := ethrunner.NewRunner(ctx, logger, spec)
		if runnerErr != nil {
			return nil, fmt.Errorf("failed to create ethereum runner: %w", runnerErr)
		}
		runner = ethRunner

	case CosmosKind:
		cosmosRunner, runnerErr := cosmosrunner.NewRunner(ctx, spec)
		if runnerErr != nil {
			return nil, fmt.Errorf("failed to create cosmos runner: %w", runnerErr)
		}
		runner = cosmosRunner

	default:
		return nil, fmt.Errorf("unsupported kind: %s", spec.Kind)
	}

	return &LoadTest{
		runner: runner,
		kind:   spec.Kind,
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

	lt.runner.PrintResults(results)

	logger.Info("load test run completed, saving results")

	if saveErr := results.Save(logger); saveErr != nil {
		return results, fmt.Errorf("failed to save results: %w", saveErr)
	}

	return results, err
}
