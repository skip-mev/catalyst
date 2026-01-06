package config

import (
	"context"
	"os"
	"strconv"
)

type Env struct {
	DevLogging         bool
	ConcurrentReceipts bool
}

type Config struct {
	ConfigPath string
}

const (
	// EnvDevLogging enabled verbose & console logging
	EnvDevLogging = "DEV_LOGGING"

	// EnvConcurrentReceipts enables concurrent tx receipt collection
	// as currently our evm rpc doesn't support eth_getBlockReceipts
	EnvConcurrentReceipts = "CATALYST_CONCURRENT_RECEIPTS"
)

type envContextKey struct{}

func ParseEnv() Env {
	return Env{
		DevLogging:         boolEnv(EnvDevLogging),
		ConcurrentReceipts: boolEnv(EnvConcurrentReceipts),
	}
}

func WithEnv(ctx context.Context, env Env) context.Context {
	return context.WithValue(ctx, envContextKey{}, env)
}

func EnvFromContext(ctx context.Context) Env {
	if env, ok := ctx.Value(envContextKey{}).(Env); ok {
		return env
	}

	return Env{}
}

func boolEnv(key string) bool {
	b, _ := strconv.ParseBool(os.Getenv(key))
	return b
}
