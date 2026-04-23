package test

import (
	// "context"
	"encoding/json"
	// "errors"
	// "testing"

	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"

	"pps-services-consumer-database/internal/model"
	// "pps-services-consumer-database/internal/usecase"
)

// helper to build a CallbackEvent with source and data.
func makeEvent(id, queueName, source string, data any) *model.TransactionEvent {
	dataBytes, _ := json.Marshal(data)
	bodyBytes, _ := json.Marshal(model.TransactionPayload{
		Source: source,
		Data:   dataBytes,
	})
	return &model.TransactionEvent{
		Id:        id,
		QueueName: queueName,
		Payload:   bodyBytes,
	}
}

// func newTestUseCase(repo *MockRepository, ds *MockDownstreamClient) *usecase.TransactionUseCase {
// 	return usecase.NewTransactionUseCase(repo, ds, newTestValidator(), newTestLogger())
// }

// // TestHandleConsumedMessage_Provider_Topup — source=PROVIDER, non-game queue.
// func TestHandleConsumedMessage_Provider_Topup(t *testing.T) {
// 	called := false
// 	repo := &MockRepository{
// 		CallSetTransactionStatusFn: func(_ context.Context, _ *model.TransactionEvent) (*model.SPResult, error) {
// 			called = true
// 			return &model.SPResult{ID: 1, Error: 0, Message: "OK"}, nil
// 		},
// 	}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("1", "biller-telkomsel-1", model.SourceProvider, model.TopupPayload{
// 		MsgId: 1, StatusToBe: "S", ClientNumber: "08123", QueueName: "biller-telkomsel-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.NoError(t, err)
// 	assert.True(t, called, "CallSetTransactionStatus should be called")
// }

// // TestHandleConsumedMessage_Provider_Game — source=PROVIDER, game queue.
// func TestHandleConsumedMessage_Provider_Game(t *testing.T) {
// 	called := false
// 	repo := &MockRepository{
// 		CallSetTransactionStatus2Fn: func(_ context.Context, _ *model.TransactionEvent) (*model.SPResult, error) {
// 			called = true
// 			return &model.SPResult{ID: 2, Error: 0, Message: "OK"}, nil
// 		},
// 	}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("2", "biller-game-1", model.SourceProvider, model.TopupPayload{
// 		MsgId: 2, StatusToBe: "S", ClientNumber: "08123", QueueName: "biller-game-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.NoError(t, err)
// 	assert.True(t, called, "CallSetTransactionStatus2 should be called for game queue")
// }

// // TestHandleConsumedMessage_Order_Success — source=ORDER, all 3 steps succeed.
// func TestHandleConsumedMessage_Order_Success(t *testing.T) {
// 	preOrderResult := &model.PreOrderResult{
// 		Imsi: "imsi1", RemarkImsi: "remark1", Mid: "mid1",
// 		StoreId: 10, QueueName: "q1", TypeVoucher: "tv1",
// 		BID: 5, TypeOfStock: "stock1", Provider: "prov1",
// 	}
// 	spOK := &model.SPResult{ID: 0, Error: 0, Message: "OK"}
// 	jualResult := &model.SPResult{ID: 1, Error: 0, Message: "VC001"}

// 	var capturedReq *model.DownstreamRequest
// 	repo := &MockRepository{
// 		CallUpdPreOrderConsumeFn: func(_ context.Context, _ *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
// 			return preOrderResult, spOK, nil
// 		},
// 		CallRequest2JualRandomWithIDFn: func(_ context.Context, _ *model.TransactionEvent) (*model.SPResult, error) {
// 			return jualResult, nil
// 		},
// 	}
// 	ds := &MockDownstreamClient{
// 		SendOrderResultFn: func(_ context.Context, req *model.DownstreamRequest) error {
// 			capturedReq = req
// 			return nil
// 		},
// 	}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("3", "order-queue", model.SourceOrder, model.OrderPayload{
// 		MsgId: 300, ConsumeStatus: "PENDING", ClientNumber: "08999", QueueName: "order-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.NoError(t, err)
// 	require.NotNil(t, capturedReq)

// 	assert.Equal(t, 300, capturedReq.MsgId)
// 	assert.Equal(t, "08999", capturedReq.ClientNumber)
// 	assert.Equal(t, "imsi1", capturedReq.Imsi)
// 	assert.Equal(t, "VC001", capturedReq.VoucherCode)
// 	assert.Equal(t, "order-queue", capturedReq.MQTransaction)
// 	assert.Equal(t, "prov1", capturedReq.Provider)
// }

// // TestHandleConsumedMessage_Order_PreOrderFails — step 1 fails.
// func TestHandleConsumedMessage_Order_PreOrderFails(t *testing.T) {
// 	repo := &MockRepository{
// 		CallUpdPreOrderConsumeFn: func(_ context.Context, _ *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
// 			return nil, nil, errors.New("db connection lost")
// 		},
// 	}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("4", "order-queue", model.SourceOrder, model.OrderPayload{
// 		MsgId: 400, ConsumeStatus: "PENDING", ClientNumber: "08111", QueueName: "order-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "CallUpdPreOrderConsume failed")
// }

// // TestHandleConsumedMessage_Order_PreOrderSPError — step 1 SP returns error code.
// func TestHandleConsumedMessage_Order_PreOrderSPError(t *testing.T) {
// 	repo := &MockRepository{
// 		CallUpdPreOrderConsumeFn: func(_ context.Context, _ *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
// 			return &model.PreOrderResult{}, &model.SPResult{Error: 99, Message: "not found"}, nil
// 		},
// 	}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("4b", "order-queue", model.SourceOrder, model.OrderPayload{
// 		MsgId: 401, ConsumeStatus: "PENDING", ClientNumber: "08111", QueueName: "order-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "CallUpdPreOrderConsume SP error")
// }

// // TestHandleConsumedMessage_Order_JualFails — step 2 fails.
// func TestHandleConsumedMessage_Order_JualFails(t *testing.T) {
// 	repo := &MockRepository{
// 		CallUpdPreOrderConsumeFn: func(_ context.Context, _ *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
// 			return &model.PreOrderResult{}, &model.SPResult{Error: 0, Message: "OK"}, nil
// 		},
// 		CallRequest2JualRandomWithIDFn: func(_ context.Context, _ *model.TransactionEvent) (*model.SPResult, error) {
// 			return nil, errors.New("SP timeout")
// 		},
// 	}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("5", "order-queue", model.SourceOrder, model.OrderPayload{
// 		MsgId: 500, ConsumeStatus: "PENDING", ClientNumber: "08222", QueueName: "order-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "CallRequest2JualRandomWithID failed")
// }

// // TestHandleConsumedMessage_Order_DownstreamFails — step 3 fails.
// func TestHandleConsumedMessage_Order_DownstreamFails(t *testing.T) {
// 	repo := &MockRepository{
// 		CallUpdPreOrderConsumeFn: func(_ context.Context, _ *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
// 			return &model.PreOrderResult{}, &model.SPResult{Error: 0, Message: "OK"}, nil
// 		},
// 		CallRequest2JualRandomWithIDFn: func(_ context.Context, _ *model.TransactionEvent) (*model.SPResult, error) {
// 			return &model.SPResult{Error: 0, Message: "VC002"}, nil
// 		},
// 	}
// 	ds := &MockDownstreamClient{
// 		SendOrderResultFn: func(_ context.Context, _ *model.DownstreamRequest) error {
// 			return errors.New("connection refused")
// 		},
// 	}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("6", "order-queue", model.SourceOrder, model.OrderPayload{
// 		MsgId: 600, ConsumeStatus: "PENDING", ClientNumber: "08333", QueueName: "order-1",
// 	})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "downstream SendOrderResult failed")
// }

// // TestHandleConsumedMessage_UnknownSource — unknown source returns error.
// func TestHandleConsumedMessage_UnknownSource(t *testing.T) {
// 	repo := &MockRepository{}
// 	ds := &MockDownstreamClient{}

// 	uc := newTestUseCase(repo, ds)

// 	event := makeEvent("7", "some-queue", "UNKNOWN", map[string]any{"msg_id": 700})

// 	err := uc.HandleConsumedMessage(context.Background(), event)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "unknown source: UNKNOWN")
// }
