package runner

import (
	"context"

	inttypes "github.com/skip-mev/catalyst/pkg/types"
)

type Runner interface {
	Run(ctx context.Context) (inttypes.LoadTestResult, error)
	PrintResults(inttypes.LoadTestResult)
}
