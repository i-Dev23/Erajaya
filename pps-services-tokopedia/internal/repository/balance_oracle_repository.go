package repository

import (
	"context"
	"database/sql"
	"fmt"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

type BalanceOracleRepository struct {
	oracleService service.OracleService
	logger        service.Logger
}

// NewBalanceOracleRepository creates a new BalanceOracleRepository with the given OracleService
func NewBalanceOracleRepository(oracleService service.OracleService, logger service.Logger) domain.BalanceRepository {
	return &BalanceOracleRepository{
		oracleService: oracleService,
		logger:        logger,
	}
}

// GetDepositBalanceCredit calls PKG_TOKPED_PRODUCT.getDepositBalanceCredit to get deposit balance information
// Example: SELECT outerrcode, outerrmsg, deposit_balance, deposit_group_name FROM TABLE(PKG_TOKPED_PRODUCT.getDepositBalanceCredit('USERTOKPEDDEV'))
func (r *BalanceOracleRepository) GetDepositBalanceCredit(ctx context.Context, username string) (*domain.BalanceResponseDomain, error) {
	query := `SELECT outerrcode, outerrmsg, deposit_balance, deposit_group_name FROM TABLE(PKG_TOKPED_PRODUCT.getDepositBalanceCredit(:1))`
	r.logger.Info("Executing balance query", "query", query, "username", username)

	rows, err := r.oracleService.Query(ctx, query, username)
	if err != nil {
		r.logger.Error("Failed to execute balance query", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.getDepositBalanceCredit: %w", err)
	}
	defer rows.Close()

	var response domain.BalanceResponseDomain

	if rows.Next() {
		var outErrCode, outErrMsg, depositGroupName sql.NullString
		var depositBalance sql.NullFloat64

		if err := rows.Scan(&outErrCode, &outErrMsg, &depositBalance, &depositGroupName); err != nil {
			r.logger.Error("Failed to scan balance result", "error", err)
			return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.getDepositBalanceCredit: %w", err)
		}

		// Map NULL values to default values
		response.OutErrCode = outErrCode.String
		response.OutErrMsg = outErrMsg.String
		response.DepositBalance = depositBalance.Float64
		response.DepositGroupName = depositGroupName.String

		r.logger.Info("Balance query successful",
			"outerrcode", response.OutErrCode,
			"outerrmsg", response.OutErrMsg,
			"deposit_balance", response.DepositBalance,
			"deposit_group_name", response.DepositGroupName)
	} else {
		r.logger.Warn("No balance data returned from Oracle", "username", username)
		return nil, fmt.Errorf("no balance data returned for username: %s", username)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating balance rows", "error", err)
		return nil, fmt.Errorf("error iterating balance rows: %w", err)
	}

	return &response, nil
}
