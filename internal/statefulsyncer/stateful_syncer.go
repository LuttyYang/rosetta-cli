package statefulsyncer

import (
	"context"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-cli/internal/logger"
	"github.com/coinbase/rosetta-cli/internal/storage"

	"github.com/coinbase/rosetta-sdk-go/fetcher"
	"github.com/coinbase/rosetta-sdk-go/syncer"
	"github.com/coinbase/rosetta-sdk-go/types"
)

var _ syncer.Handler = (*StatefulSyncer)(nil)

type StatefulSyncer struct {
	network        *types.NetworkIdentifier
	fetcher        *fetcher.Fetcher
	cancel         context.CancelFunc
	blockStorage   *storage.BlockStorage
	counterStorage *storage.CounterStorage
	logger         *logger.Logger
	syncer         *syncer.Syncer
	workers        []storage.BlockWorker
}

func New(
	ctx context.Context,
	network *types.NetworkIdentifier,
	fetcher *fetcher.Fetcher,
	blockStorage *storage.BlockStorage,
	counterStorage *storage.CounterStorage,
	logger *logger.Logger,
	cancel context.CancelFunc,
	workers []storage.BlockWorker,
) *StatefulSyncer {
	return &StatefulSyncer{
		network:        network,
		fetcher:        fetcher,
		cancel:         cancel,
		blockStorage:   blockStorage,
		counterStorage: counterStorage,
		workers:        workers,
		logger:         logger,
	}
}

func (s *StatefulSyncer) Sync(ctx context.Context, startIndex int64, endIndex int64) error {
	// Ensure storage is in correct state for starting at index
	if startIndex != -1 { // attempt to remove blocks from storage (without handling)
		if err := s.blockStorage.SetNewStartIndex(ctx, startIndex, s.workers); err != nil {
			return fmt.Errorf("%w: unable to set new start index", err)
		}
	} else { // attempt to load last processed index
		head, err := s.blockStorage.GetHeadBlockIdentifier(ctx)
		if err == nil {
			startIndex = head.Index + 1
		}
	}

	// Load in previous blocks into syncer cache to handle reorgs.
	// If previously processed blocks exist in storage, they are fetched.
	// Otherwise, none are provided to the cache (the syncer will not attempt
	// a reorg if the cache is empty).
	pastBlocks := s.blockStorage.CreateBlockCache(ctx)

	syncer := syncer.New(
		s.network,
		s.fetcher,
		s,
		s.cancel,
		pastBlocks,
	)

	return syncer.Sync(ctx, startIndex, endIndex)
}

func (s *StatefulSyncer) BlockAdded(ctx context.Context, block *types.Block) error {
	err := s.blockStorage.AddBlock(ctx, block, s.workers)
	if err != nil {
		return fmt.Errorf("%w: unable to add block to storage %s:%d", err, block.BlockIdentifier.Hash, block.BlockIdentifier.Index)
	}

	if err := s.logger.AddBlockStream(ctx, block); err != nil {
		return nil
	}

	// Update Counters
	_, _ = s.counterStorage.Update(ctx, storage.BlockCounter, big.NewInt(1))
	_, _ = s.counterStorage.Update(
		ctx,
		storage.TransactionCounter,
		big.NewInt(int64(len(block.Transactions))),
	)
	opCount := int64(0)
	for _, txn := range block.Transactions {
		opCount += int64(len(txn.Operations))
	}
	_, _ = s.counterStorage.Update(ctx, storage.OperationCounter, big.NewInt(opCount))

	return nil
}

func (s *StatefulSyncer) BlockRemoved(ctx context.Context, blockIdentifier *types.BlockIdentifier) error {
	err := s.blockStorage.RemoveBlock(ctx, blockIdentifier, s.workers)
	if err != nil {
		return fmt.Errorf("%w: unable to remove block from storage %s:%d", err, blockIdentifier.Hash, blockIdentifier.Index)
	}

	if err := s.logger.RemoveBlockStream(ctx, blockIdentifier); err != nil {
		return nil
	}

	// Update Counters
	_, _ = s.counterStorage.Update(ctx, storage.OrphanCounter, big.NewInt(1))

	return err
}
