package usecase

import (
	"context"
	"pps-services-tokopedia/internal/config"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strings"
	"time"
)

type balanceUsecaseImpl struct {
	config      *config.Config
	logger      service.Logger
	balanceRepo domain.BalanceRepository
}

func NewBalanceUsecase(
	config *config.Config,
	logger service.Logger,
	balanceRepo domain.BalanceRepository,
) domain.BalanceUsecase {
	return &balanceUsecaseImpl{
		config:      config,
		logger:      logger,
		balanceRepo: balanceRepo,
	}
}

func (u *balanceUsecaseImpl) GetBalance(ctx context.Context, req *domain.BalanceGetRequestDomain) (*domain.BalanceGetResponseDomain, error) {
	u.logger.Info("Balance request received", "timestamp", req.Timestamp)

	if strings.TrimSpace(req.Timestamp) == "" {
		rc, _ := utils.GetResponseCode("42")
		return &domain.BalanceGetResponseDomain{
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	username := ""
	if u.config != nil {
		username = strings.TrimSpace(u.config.TPClientID)
	}
	if username == "" {
		username = strings.TrimSpace(utils.GetEnv("TP_CLIENT_ID", ""))
	}
	if username == "" {
		username = "USERTOKPEDDEV"
	}

	oracleResp, err := u.balanceRepo.GetDepositBalanceCredit(ctx, username)
	if err != nil {
		u.logger.Error("Failed to get deposit balance credit", "error", err, "username", username)
		rc, _ := utils.GetResponseCode("62")
		return &domain.BalanceGetResponseDomain{
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}, err
	}

	// Oracle outerrcode convention: "0" = success
	if strings.TrimSpace(oracleResp.OutErrCode) != "0" {
		u.logger.Warn("Oracle returned non-success outerrcode",
			"outerrcode", oracleResp.OutErrCode,
			"outerrmsg", oracleResp.OutErrMsg,
			"username", username)
		rc, _ := utils.GetResponseCode("62")
		msg := rc.Message
		if strings.TrimSpace(oracleResp.OutErrMsg) != "" {
			msg = oracleResp.OutErrMsg
		}
		return &domain.BalanceGetResponseDomain{
			ResponseCode: rc.Code,
			Message:      msg,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	label := strings.TrimSpace(oracleResp.DepositGroupName)
	if label == "" {
		label = "General"
	}

	rc, _ := utils.GetResponseCode("00")
	return &domain.BalanceGetResponseDomain{
		ResponseCode: rc.Code,
		Message:      rc.Message,
		BalanceDetails: []domain.BalanceDetail{
			{Label: label, Value: oracleResp.DepositBalance},
		},
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}
