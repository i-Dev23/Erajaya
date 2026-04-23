package test

// import (
// 	"context"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
// 	"time"

// 	"github.com/rs/zerolog"
// 	"github.com/spf13/viper"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"

// 	"pps-services-consumer-database/internal/gateway/downstream"
// 	"pps-services-consumer-database/internal/model"
// )

// func newDownstreamClient(url string, timeout int) *downstream.Client {
// 	v := viper.New()
// 	v.Set("downstream.url", url)
// 	v.Set("downstream.timeout", timeout)
// 	log := zerolog.Nop()
// 	return downstream.NewClient(v, log)
// }

// // TestSendOrderResult_Success — mock HTTP server returns 200.
// func TestSendOrderResult_Success(t *testing.T) {
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		assert.Equal(t, http.MethodPost, r.Method)
// 		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
// 		w.WriteHeader(http.StatusOK)
// 	}))
// 	defer server.Close()

// 	client := newDownstreamClient(server.URL, 5)

// 	req := &model.DownstreamRequest{
// 		MsgId:        1,
// 		ClientNumber: "08123",
// 		Imsi:         "imsi1",
// 		VoucherCode:  "VC001",
// 	}

// 	err := client.SendOrderResult(context.Background(), req)
// 	require.NoError(t, err)
// }

// // TestSendOrderResult_ServerError — mock HTTP server returns 500.
// func TestSendOrderResult_ServerError(t *testing.T) {
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
// 		w.WriteHeader(http.StatusInternalServerError)
// 	}))
// 	defer server.Close()

// 	client := newDownstreamClient(server.URL, 5)

// 	req := &model.DownstreamRequest{MsgId: 2}

// 	err := client.SendOrderResult(context.Background(), req)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "downstream returned status 500")
// }

// // TestSendOrderResult_Timeout — mock HTTP server delays beyond timeout.
// func TestSendOrderResult_Timeout(t *testing.T) {
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
// 		time.Sleep(3 * time.Second)
// 		w.WriteHeader(http.StatusOK)
// 	}))
// 	defer server.Close()

// 	client := newDownstreamClient(server.URL, 1) // 1 second timeout

// 	req := &model.DownstreamRequest{MsgId: 3}

// 	err := client.SendOrderResult(context.Background(), req)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "downstream request failed")
// }
