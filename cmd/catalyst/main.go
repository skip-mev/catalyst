package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/skip-mev/catalyst/chains"
	logging "github.com/skip-mev/catalyst/chains/log"
	"github.com/skip-mev/catalyst/chains/types"
	"github.com/skip-mev/catalyst/config"
)

var errFailed = errors.New("failure")

func main() {
	var (
		env  = config.ParseEnv()
		args = parseArgs()
		ctx  = context.Background()
	)

	logger, _ := logging.DefaultLogger(env.DevLogging)
	defer logging.CloseLogFile()

	ctx = config.WithEnv(ctx, env)
	ctx = logging.WithLogger(ctx, logger)

	exitIfErr := func(err error, message string) {
		if err == nil {
			return
		}

		err = errors.Wrap(err, message)
		saveConfigError(err, logger)
		logger.Fatal("Failure", zap.Error(err))
	}

	if args.ConfigPath == "" {
		exitIfErr(errFailed, "config file path is required")
	}

	data, err := os.ReadFile(args.ConfigPath)
	exitIfErr(err, "failed to read config file")

	// unmarshal into the single shared spec (with custom UnmarshalYAML).
	var spec types.LoadTestSpec
	err = yaml.Unmarshal(data, &spec)
	exitIfErr(err, "failed to parse config file")
	exitIfErr(spec.Validate(), "failed to validate config file")

	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	if kind == "" {
		exitIfErr(errFailed, "config is missing required field 'kind'")
	}

	test, err := chains.NewLoadTest(ctx, logger, spec)
	exitIfErr(err, fmt.Sprintf("failed to create %s test", kind))

	if _, err = test.Run(ctx, logger); err != nil {
		logger.Fatal("failed to run load test", zap.Error(err))
	}
}

func parseArgs() config.Config {
	configPath := flag.String("config", "", "Path to load test configuration file")
	flag.Parse()

	return config.Config{
		ConfigPath: *configPath,
	}
}

func saveConfigError(err error, logger *zap.Logger) {
	out := types.LoadTestResult{
		Error: err.Error(),
	}

	if errSave := chains.SaveResults(out, logger); errSave != nil {
		logger.Error("failed to save results", zap.Error(errSave))
	}
}
