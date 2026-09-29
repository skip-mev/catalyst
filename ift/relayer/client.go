package relayer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	relayerv2 "github.com/cosmos/ibc/cli/api/v2/relayer"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	maxRelayRetries = 15
	relayRetryDelay = 3 * time.Second
)

type Client interface {
	SubmitTxHash(ctx context.Context, txHash string) error
}

type GRPCClient struct {
	client  relayerv2.RelayerApiServiceClient
	chainID string
	timeout time.Duration
	metrics *Metrics
}

func NewGRPCClient(cfg loadtesttypes.RelayConfig, chainID string, metrics *Metrics) (*GRPCClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &GRPCClient{
		client: relayerv2.NewRelayerApiServiceClient(
			newH2CClient(),
			baseURL(cfg.URL),
			connect.WithGRPC(),
		),
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
		_, err := c.client.Relay(callCtx, connect.NewRequest(&relayerv2.RelayRequest{
			TxHash:        txHash,
			SourceChainId: c.chainID,
			Selection: &relayerv2.RelayRequest_AllPackets{
				AllPackets: &relayerv2.AllPackets{},
			},
		}))
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
	return nil
}

func baseURL(raw string) string {
	if strings.Contains(raw, "://") {
		return strings.TrimRight(raw, "/")
	}
	return "http://" + raw
}

// h2c matches the IBC CLI client: the relayer serves gRPC on plaintext HTTP/2.
func newH2CClient() *http.Client {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Client{
		Transport: &http.Transport{Protocols: protocols},
	}
}
