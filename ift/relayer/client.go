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

type Client interface {
	SubmitTxHash(ctx context.Context, txHash string) error
}

type GRPCClient struct {
	conn    *grpc.ClientConn
	client  relayerapi.RelayerApiServiceClient
	chainID string
	timeout time.Duration
}

func NewGRPCClient(cfg loadtesttypes.IFTRelayerConfig, chainID string) (*GRPCClient, error) {
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
	}, nil
}

func (c *GRPCClient) SubmitTxHash(ctx context.Context, txHash string) error {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.Relay(callCtx, &relayerapi.RelayRequest{
		TxHash:  txHash,
		ChainId: c.chainID,
	})
	if err != nil {
		return fmt.Errorf("submit tx hash to relayer: %w", err)
	}

	return nil
}

func (c *GRPCClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}
