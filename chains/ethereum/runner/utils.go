package runner

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"

	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

func getTxType(tx *gethtypes.Transaction) loadtesttypes.MsgType {
	if tx.To() == nil {
		return inttypes.ContractCreate
	}
	return inttypes.ContractCall
}

func WriteTxnsToCache(name string, txs [][]*gethtypes.Transaction) error {
	//nolint:gosec // G302
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o777)
	if err != nil {
		return fmt.Errorf("could not open cache file %s: %w", name, err)
	}
	defer f.Close()

	// assumes the number of txs in each batch is equal
	numTxs := len(txs) * len(txs[0])
	if _, err := f.WriteString(strconv.Itoa(numTxs) + "\n"); err != nil {
		return fmt.Errorf("writing header of num txs to file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	zw := gzip.NewWriter(f)
	defer zw.Close()

	writer := bufio.NewWriter(zw)
	defer writer.Flush()

	for _, batch := range txs {
		for _, tx := range batch {
			bn, err := tx.MarshalBinary()
			if err != nil {
				return fmt.Errorf("binary marshalling tx: %w", err)
			}

			enc := base64.RawStdEncoding.EncodeToString(bn)
			if _, err := writer.WriteString(enc + "\n"); err != nil {
				return fmt.Errorf("writing tx binary to file: %w", err)
			}
		}
	}

	return nil
}

func waitForEmptyMempool(ctx context.Context, clients []*ethclient.Client, logger *zap.Logger, timeout time.Duration) {
	wg := sync.WaitGroup{}
	for _, c := range clients {
		wg.Add(1)
		go func(ethClient *ethclient.Client) {
			client := ethClient.Client()
			defer wg.Done()
			type TxPoolStatus struct {
				Pending hexutil.Uint64 `json:"pending"`
				Queued  hexutil.Uint64 `json:"queued"`
			}
			type TxPoolStatusResponse struct {
				JSONRPC string       `json:"jsonrpc"`
				ID      int          `json:"id"`
				Result  TxPoolStatus `json:"result"`
			}

			started := time.Now()
			timer := time.NewTicker(500 * time.Millisecond)
			timout := time.NewTimer(timeout)
			defer timer.Stop()
			defer timout.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					var res TxPoolStatusResponse
					err := client.CallContext(ctx, &res, "txpool_status")
					if err == nil {
						if res.Result.Pending == 0 {
							logger.Debug(
								"mempool clear. done waiting for mempool",
								zap.Duration("waited", time.Since(started)),
							)
							return
						}
					} else {
						logger.Debug("error calling txpool status", zap.Error(err))
					}
				case <-timout.C:
					logger.Debug("timed out waiting for mempool to clear", zap.Duration("waited", timeout))
					return
				}
			}
		}(c)
	}
	wg.Wait()
}

func ReadTxnsFromCache(name string, numBatches int) ([][]*gethtypes.Transaction, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("could not open cache file %s: %w", name, err)
	}
	defer f.Close()

	header, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading file header: %w", err)
	}

	numTxs, err := strconv.Atoi(strings.TrimSuffix(header, "\n"))
	if err != nil {
		return nil, fmt.Errorf("converting header to int: %w", err)
	}
	batchSize := numTxs / numBatches

	if _, err := f.Seek(int64(len(header)), 0); err != nil {
		return nil, fmt.Errorf("seeking: %w", err)
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer zr.Close()

	reader := bufio.NewReader(zr)

	batches := make([][]*gethtypes.Transaction, numBatches)
	batchIdx := 0
	i := 1
	for {
		// read a line, which is a gzipped, base64 encoded, binary encoded tx
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return batches, nil
			}
			return nil, fmt.Errorf("reading line: %w", err)
		}
		if len(line) == 0 {
			return batches, nil
		}
		if batchIdx == numBatches {
			// this can happen if they have specified an amount of batches that
			// does not divide evenly into the total amount of txns in the
			// cache. in this case we simply return early (without bringing all
			// of the txs in the cache into their batches), so we respect the
			// amount of batches that they wanted and do not create extras.
			return batches, nil
		}
		line = line[0 : len(line)-1] // remove trailing \n

		bz, err := base64.RawStdEncoding.DecodeString(line)
		if err != nil {
			return nil, fmt.Errorf("base64 decoding: %w", err)
		}

		var tx gethtypes.Transaction
		if err := tx.UnmarshalBinary(bz); err != nil {
			return nil, fmt.Errorf("unmarshal binary tx: %w", err)
		}

		batches[batchIdx] = append(batches[batchIdx], &tx)
		if i%batchSize == 0 {
			batchIdx++
		}
		i++
	}
}
