package service

import "context"

// GameSyncService defines the interface for syncing game list data.
type GameSyncService interface {
	SyncGameList(ctx context.Context) error
	SyncVoucherList(ctx context.Context) error
	SyncSingleGame(ctx context.Context, gameCode string) error
	SyncSingleDenomination(ctx context.Context, gameCode string, denominationID int) error
}
