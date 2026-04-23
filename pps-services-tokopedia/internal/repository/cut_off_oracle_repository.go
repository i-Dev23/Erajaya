package repository

import (
	"context"
	"database/sql"
	"errors"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"strings"
)

type CutOffOracleRepository struct {
	oracleService service.OracleService
	logger        service.Logger
}

func NewCutOffOracleRepository(oracleService service.OracleService, logger service.Logger) domain.CutOffRepository {
	return &CutOffOracleRepository{
		oracleService: oracleService,
		logger:        logger,
	}
}

// BEGIN PRC_CUT_OFF_N_MAINTAIN(InFlag=>:1,OutErrCode=>:2,OutMsgErr=>:3); END;
func (r *CutOffOracleRepository) CutOff(ctx context.Context, flag string) (*domain.CutOffResponseDomain, error) {
	query := `BEGIN PRC_CUT_OFF_N_MAINTAIN(InFlag=>:1,OutErrCode=>:2,OutMsgErr=>:3); END;`
	errCode := strings.Repeat(" ", 4000)
	errMsg := strings.Repeat(" ", 4000)
	_, err := r.oracleService.Exec(ctx, query, flag,
		sql.Out{Dest: &errCode},
		sql.Out{Dest: &errMsg},
	)
	if err != nil {
		r.logger.Error("Failed to execute query", "error", err, "query", query, "req", flag)
		return nil, errors.New("failed to execute PRC_CUT_OFF_N_MAINTAIN")
	}
	return &domain.CutOffResponseDomain{
		OutErrCode: errCode,
		OutMsgErr:  errMsg,
	}, nil
}
