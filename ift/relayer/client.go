package relayer

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	relayerapi "github.com/skip-mev/catalyst/ift/relayer/pb/relayerapi"
)

const (
	maxRelayRetries = 15
	relayRetryDelay = 3 * time.Second
)

type Client interface {
	SubmitTxHash(ctx context.Context, txHash string) error
}

type GRPCClient struct {
	conn    *grpc.ClientConn
	client  relayerapi.RelayerApiServiceClient
	chainID string
	timeout time.Duration
	metrics *Metrics
}

func NewGRPCClient(cfg loadtesttypes.RelayConfig, chainID string, metrics *Metrics) (*GRPCClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	conn, err := grpc.NewClient(
		cfg.URL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create relayer grpc client: %w", err)
	}

	return &GRPCClient{
		conn:    conn,
		client:  relayerapi.NewRelayerApiServiceClient(conn),
		chainID: chainID,
		timeout: timeout,
		metrics: metrics,
	}, nil
}

func (c *GRPCClient) SubmitTxHash(ctx context.Context, txHash string) error {
	var lastErr error
	for attempt := range maxRelayRetries {
		if attempt > 0 {
			timer := time.NewTimer(relayRetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				if c.metrics != nil {
					c.metrics.Failure.WithLabelValues(c.chainID).Inc()
				}
				return ctx.Err()
			case <-timer.C:
			}
		}

		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		start := time.Now()
		_, err := c.client.Relay(callCtx, &relayerapi.RelayRequest{
			TxHash:  txHash,
			ChainId: c.chainID,
		})
		cancel()
		if c.metrics != nil {
			c.metrics.Duration.WithLabelValues(c.chainID).Observe(time.Since(start).Seconds())
		}

		if err == nil {
			if c.metrics != nil {
				c.metrics.Success.WithLabelValues(c.chainID).Inc()
			}
			return nil
		}
		lastErr = err
	}

	if c.metrics != nil {
		c.metrics.Failure.WithLabelValues(c.chainID).Inc()
	}
	return fmt.Errorf("submit tx hash to relayer after %d attempts: %w", maxRelayRetries, lastErr)
}

func (c *GRPCClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}
