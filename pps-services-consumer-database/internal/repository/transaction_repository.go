package repository

import (
	"context"
	"database/sql"
	"fmt"
	"pps-services-consumer-database/internal/model"
	"strconv"

	"github.com/rs/zerolog"
	go_ora "github.com/sijms/go-ora/v2"
)

type TransactionRepository struct {
	oracleDB *sql.DB
	log      zerolog.Logger
}

func NewTransactionRepository(oracleDB *sql.DB, log zerolog.Logger) *TransactionRepository {
	return &TransactionRepository{
		oracleDB: oracleDB,
		log:      log,
	}
}

// // decodeBody decodes event.Body (json.RawMessage) into a map using UseNumber to preserve int types.
// func decodeBody(raw json.RawMessage) (map[string]any, error) {
// 	var body map[string]any
// 	dec := json.NewDecoder(bytes.NewReader(raw))
// 	dec.UseNumber()
// 	if err := dec.Decode(&body); err != nil {
// 		return nil, fmt.Errorf("failed to decode body: %w", err)
// 	}
// 	return body, nil
// }

// CallSetTransactionStatus executes the Oracle SP MSG.SETTRANSACTIONSTATUS.
func (r *TransactionRepository) CallSetTransactionStatus(ctx context.Context, event *model.TransactionEvent, payload *model.TopupDataPayload) (*model.SPResult, error) {
	// body, err := decodeBody(event.Payload)
	// if err != nil {
	// 	return nil, nil, err
	// }
	var outID, outError int
	var outMessage string

	msgID, err := strconv.Atoi(payload.MsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert '%s' to int: %w", payload.MsgID, err)
	}

	_, err = r.oracleDB.ExecContext(ctx, `BEGIN MSG.SETTRANSACTIONSTATUS(
		INMSGID                    => :1,
		INSTATUSTOBE               => :2,
		INSERIALNUMBER             => :3,
		INNUMBERBUYER              => :4,
		INNOMINAL                  => :5,
		INORIGINALCONVERSATIONID   => :6,
		INCONVERSATIONID           => :7,
		INMESAGETOCUSTOMER         => :8,
		INADDITIONALMESSAGE        => :9,
		OUTID                      => :10,
		OUTERROR                   => :11,
		OUTMESSAGE                 => :12
	); END;`, msgID,
		payload.StatusToBe,
		payload.SerialNumber,
		payload.ClientNumber,
		payload.Nominal,
		payload.OriginalConversationID,
		payload.ConversationID,
		payload.MessageToCustomer,
		payload.AdditionalMessage,
		go_ora.Out{Dest: &outID},
		go_ora.Out{Dest: &outError},
		go_ora.Out{Dest: &outMessage, Size: 4000})
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to call SP for transaction %s", event.Id)
		return nil, fmt.Errorf("failed to call SP: %w", err)
	}

	result := &model.SPResult{
		ID:      outID,
		Error:   outError,
		Message: outMessage,
	}

	// if outError != 0 {
	// 	r.log.Warn().Msgf("SP returned error for %s: %d - %s", event.Id, outError, outMessage)
	// } else {
	// 	r.log.Debug().Msgf("SP executed for %s: ID=%d", event.Id, outID)
	// }

	return result, nil
}

// CallUpdPreOrderConsume executes the Oracle SP MSG.updPreOrderConsume.
// Returns PreOrderResult with all output fields + SPResult for error checking.
func (r *TransactionRepository) CallUpdPreOrderConsume(ctx context.Context, event *model.TransactionEvent, payload *model.OrderPayload) (*model.PreOrderResult, *model.SPResult, error) {
	var outStoreId, outBID, outError int
	var outImsi, outRemarkImsi, outMid, outQueueName, outTypeVoucher, outTypeOfStock, outProvider, outMessage string

	_, err := r.oracleDB.ExecContext(ctx, `BEGIN MSG.updPreOrderConsume(
		InMsgId         => :1,
		InConsumeStatus => :2,
		OutImsi         => :3,
		OutRemarkImsi   => :4,
		OutMid          => :5,
		OutStoreId      => :6,
		OutQueueName    => :7,
		OutTypeVoucher  => :8,
		OutBID          => :9,
		OutTypeOfStock  => :10,
		OutProvider     => :11,
		OutError        => :12,
		OutMessage      => :13
	); END;`, payload.MsgID,
		payload.ConsumeStatus,
		go_ora.Out{Dest: &outImsi, Size: 4000},
		go_ora.Out{Dest: &outRemarkImsi, Size: 4000},
		go_ora.Out{Dest: &outMid, Size: 4000},
		go_ora.Out{Dest: &outStoreId},
		go_ora.Out{Dest: &outQueueName, Size: 4000},
		go_ora.Out{Dest: &outTypeVoucher, Size: 4000},
		go_ora.Out{Dest: &outBID},
		go_ora.Out{Dest: &outTypeOfStock, Size: 4000},
		go_ora.Out{Dest: &outProvider, Size: 4000},
		go_ora.Out{Dest: &outError},
		go_ora.Out{Dest: &outMessage, Size: 4000})
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to call SP for transaction %s", event.Id)
		return nil, nil, fmt.Errorf("failed to call SP: %w", err)
	}

	preOrder := &model.PreOrderResult{
		IMSI:        outImsi,
		RemarkIMSI:  outRemarkImsi,
		MID:         outMid,
		StoreID:     outStoreId,
		QueueName:   outQueueName,
		TypeVoucher: outTypeVoucher,
		BID:         outBID,
		TypeOfStock: outTypeOfStock,
		Provider:    outProvider,
	}

	spResult := &model.SPResult{
		ID:      0,
		Error:   outError,
		Message: outMessage,
	}

	// if outError != 0 {
	// 	r.log.Warn().Msgf("SP returned error for %s: %d - %s", event.Id, outError, outMessage)
	// } else {
	// 	r.log.Debug().Msgf("SP executed for %s", event.Id)
	// }

	return preOrder, spResult, nil
}

// CallRequest2JualRandomWithID executes the Oracle SP MSG.request2JualRandomWithID.
func (r *TransactionRepository) CallRequest2JualRandomWithID(ctx context.Context, event *model.TransactionEvent, payload *model.OrderPayload, preOrder *model.PreOrderResult) (*model.SPResult, error) {
	var outError int
	var outMessage string

	_, err := r.oracleDB.ExecContext(ctx, `BEGIN MSG.request2JualRandomWithID(
		p_msg_id    => :1,
		p_user      => :2,
		p_msisdn    => :3,
		p_produk    => :4,
		p_notrx     => :5,
		p_signature => :6,
		p_ip        => :7,
		p_mid       => :8
		p_id        => :9,
		p_status    => :10,
		err         => :11,
		msg         => :12
	); END;`, payload.MsgID,
		payload.User,
		payload.ClientNumber,
		payload.VoucherCode,
		payload.TrxNo,
		payload.Signature,
		payload.IP,
		preOrder.MID,
		payload.MsgID,
		payload.Status,
		go_ora.Out{Dest: &outError},
		go_ora.Out{Dest: &outMessage, Size: 4000})
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to call SP for transaction %s", event.Id)
		return nil, fmt.Errorf("failed to call SP: %w", err)
	}

	result := &model.SPResult{
		ID:      0,
		Error:   outError,
		Message: outMessage,
	}

	// if outError != 0 {
	// 	r.log.Warn().Msgf("SP returned error for %s: %d - %s", event.Id, outError, outMessage)
	// } else {
	// 	r.log.Debug().Msgf("SP executed for %s", event.Id)
	// }

	return result, nil
}

func (r *TransactionRepository) PingOracle(ctx context.Context) error {
	return r.oracleDB.PingContext(ctx)
}
