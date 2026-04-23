package gamesync

import (
	"context"
	"encoding/json"
	"fmt"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
	contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"
	"pps-services-gateway-unipin/pkg/unipin"
)

var _ contractsvc.GameSyncService = (*SyncServiceImpl)(nil)

// SyncServiceImpl syncs UniPin game list + voucher list into Oracle via stored procedures.
type SyncServiceImpl struct {
	client      *unipin.Client
	repo        repository.GameListRepository
	voucherRepo repository.VoucherListRepository
	logger      contractsvc.Logger
}

// NewSyncService creates a new SyncServiceImpl.
func NewSyncService(
	client *unipin.Client,
	repo repository.GameListRepository,
	voucherRepo repository.VoucherListRepository,
	logger contractsvc.Logger,
) *SyncServiceImpl {
	return &SyncServiceImpl{
		client:      client,
		repo:        repo,
		voucherRepo: voucherRepo,
		logger:      logger,
	}
}

// SyncGameList fetches game list, then detail per game, and upserts each denomination.
func (s *SyncServiceImpl) SyncGameList(ctx context.Context) error {
	s.logger.Info("sync game list started")

	listResp, err := s.client.GameList(ctx)
	if err != nil {
		return fmt.Errorf("fetch game list: %w", err)
	}

	s.logger.Info("game list fetched", "total_games", len(listResp.GameList))

	var totalUpserted, totalErrors int

	for _, game := range listResp.GameList {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		detail, err := s.client.GameDetail(ctx, game.GameCode)
		if err != nil {
			s.logger.Error("fetch game detail failed, skipping",
				"game_code", game.GameCode,
				"error", err,
			)
			totalErrors++
			continue
		}

		fieldJSON, _ := json.Marshal(detail.Fields)

		for _, denom := range detail.Denominations {
			row := &repository.GameListRow{
				GameCode:       game.GameCode,
				GameDesc:       game.GameName,
				GameCategory:   game.GameCategory,
				DenominationID: denom.ID,
				GameName:       denom.Name,
				Currency:       denom.Currency,
				Amount:         denom.Amount,
				FieldRequest:   string(fieldJSON),
				Provider:       "unipin-game",
			}

			errCode, errMsg, err := s.repo.UpsertGameList(ctx, row)
			if err != nil {
				s.logger.Error("upsert game list failed",
					"game_code", game.GameCode,
					"denomination_id", denom.ID,
					"error", err,
				)
				totalErrors++
				continue
			}

			if errCode != "" && errCode != "0" {
				s.logger.Warn("upsert game list sp error",
					"game_code", game.GameCode,
					"denomination_id", denom.ID,
					"err_code", errCode,
					"err_msg", errMsg,
				)
				totalErrors++
				continue
			}

			totalUpserted++
		}
	}

	s.logger.Info("sync game list completed",
		"total_upserted", totalUpserted,
		"total_errors", totalErrors,
	)

	return nil
}

// SyncSingleGame fetches detail for one game and upserts all its denominations.
func (s *SyncServiceImpl) SyncSingleGame(ctx context.Context, gameCode string) error {
	s.logger.Info("sync single game started", "game_code", gameCode)

	listResp, err := s.client.GameList(ctx)
	if err != nil {
		return fmt.Errorf("fetch game list: %w", err)
	}

	var game *unipin.Game
	for i := range listResp.GameList {
		if listResp.GameList[i].GameCode == gameCode {
			game = &listResp.GameList[i]
			break
		}
	}
	if game == nil {
		return fmt.Errorf("game_code %s not found in game list", gameCode)
	}

	detail, err := s.client.GameDetail(ctx, gameCode)
	if err != nil {
		return fmt.Errorf("fetch game detail: %w", err)
	}

	if len(detail.Denominations) == 0 {
		return fmt.Errorf("game %s has no denominations", gameCode)
	}

	fieldJSON, _ := json.Marshal(detail.Fields)
	var totalUpserted, totalErrors int

	for _, denom := range detail.Denominations {
		row := &repository.GameListRow{
			GameCode:       game.GameCode,
			GameDesc:       game.GameName,
			GameCategory:   game.GameCategory,
			DenominationID: denom.ID,
			GameName:       denom.Name,
			Currency:       denom.Currency,
			Amount:         denom.Amount,
			FieldRequest:   string(fieldJSON),
			Provider:       "unipin-game",
		}

		errCode, errMsg, err := s.repo.UpsertGameList(ctx, row)
		if err != nil {
			s.logger.Error("upsert failed", "game_code", gameCode, "denomination_id", denom.ID, "error", err)
			totalErrors++
			continue
		}
		if errCode != "" && errCode != "0" {
			s.logger.Warn("upsert sp error", "game_code", gameCode, "denomination_id", denom.ID, "err_code", errCode, "err_msg", errMsg)
			totalErrors++
			continue
		}
		totalUpserted++
	}

	s.logger.Info("sync single game completed", "game_code", gameCode, "upserted", totalUpserted, "errors", totalErrors)
	return nil
}

// SyncSingleDenomination fetches detail for a game and upserts one specific denomination.
func (s *SyncServiceImpl) SyncSingleDenomination(ctx context.Context, gameCode string, denominationID int) error {
	s.logger.Info("sync single denomination started", "game_code", gameCode, "denomination_id", denominationID)

	listResp, err := s.client.GameList(ctx)
	if err != nil {
		return fmt.Errorf("fetch game list: %w", err)
	}

	var game *unipin.Game
	for i := range listResp.GameList {
		if listResp.GameList[i].GameCode == gameCode {
			game = &listResp.GameList[i]
			break
		}
	}
	if game == nil {
		return fmt.Errorf("game_code %s not found in game list", gameCode)
	}

	detail, err := s.client.GameDetail(ctx, gameCode)
	if err != nil {
		return fmt.Errorf("fetch game detail: %w", err)
	}

	var denom *unipin.Denomination
	for i := range detail.Denominations {
		if detail.Denominations[i].ID == denominationID {
			denom = &detail.Denominations[i]
			break
		}
	}
	if denom == nil {
		return fmt.Errorf("denomination_id %d not found for game %s", denominationID, gameCode)
	}

	fieldJSON, _ := json.Marshal(detail.Fields)

	row := &repository.GameListRow{
		GameCode:       game.GameCode,
		GameDesc:       game.GameName,
		GameCategory:   game.GameCategory,
		DenominationID: denom.ID,
		GameName:       denom.Name,
		Currency:       denom.Currency,
		Amount:         denom.Amount,
		FieldRequest:   string(fieldJSON),
		Provider:       "unipin-game",
	}

	errCode, errMsg, err := s.repo.UpsertGameList(ctx, row)
	if err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}
	if errCode != "" && errCode != "0" {
		return fmt.Errorf("SP error: code=%s msg=%s", errCode, errMsg)
	}

	s.logger.Info("sync single denomination completed", "game_code", gameCode, "denomination_id", denominationID)
	return nil
}

// SyncVoucherList fetches voucher list, then detail per voucher, and upserts each denomination.
func (s *SyncServiceImpl) SyncVoucherList(ctx context.Context) error {
	s.logger.Info("sync voucher list started")

	listResp, err := s.client.VoucherList(ctx)
	if err != nil {
		return fmt.Errorf("fetch voucher list: %w", err)
	}

	s.logger.Info("voucher list fetched", "total_vouchers", len(listResp.VoucherList))

	var totalUpserted, totalErrors int

	for _, voucher := range listResp.VoucherList {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		detail, err := s.client.VoucherDetail(ctx, voucher.VoucherCode)
		if err != nil {
			s.logger.Error("fetch voucher detail failed, skipping",
				"voucher_code", voucher.VoucherCode,
				"error", err,
			)
			totalErrors++
			continue
		}

		for _, denom := range detail.Denominations {
			row := &repository.VoucherListRow{
				VoucherCode:             voucher.VoucherCode,
				VoucherDesc:             voucher.VoucherName,
				VoucherCategory:         "",
				VoucherDenominationCode: denom.DenominationCode,
				VoucherName:             denom.DenominationName,
				Currency:                denom.DenominationCurrency,
				Amount:                  denom.DenominationAmount,
				Provider:                "unipin-voucher",
			}

			errCode, errMsg, err := s.voucherRepo.UpsertVoucherList(ctx, row)
			if err != nil {
				s.logger.Error("upsert voucher list failed",
					"voucher_code", voucher.VoucherCode,
					"denomination_code", denom.DenominationCode,
					"error", err,
				)
				totalErrors++
				continue
			}

			if errCode != "" && errCode != "0" {
				s.logger.Warn("upsert voucher list sp error",
					"voucher_code", voucher.VoucherCode,
					"denomination_code", denom.DenominationCode,
					"err_code", errCode,
					"err_msg", errMsg,
				)
				totalErrors++
				continue
			}

			totalUpserted++
		}
	}

	s.logger.Info("sync voucher list completed",
		"total_upserted", totalUpserted,
		"total_errors", totalErrors,
	)

	return nil
}
