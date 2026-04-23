package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pps-services-consumer-database/internal/config"
	"pps-services-consumer-database/internal/gateway/downstream"
	"pps-services-consumer-database/internal/gateway/messaging"
	"pps-services-consumer-database/internal/model"
	"pps-services-consumer-database/internal/repository"
	"pps-services-consumer-database/internal/usecase"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	validate := config.NewValidator(viperConfig)

	oracleDB := config.NewOracleDB(viperConfig, log)

	rabbitMQ := config.NewRabbitMQ(viperConfig, log)

	transactionRepo := repository.NewTransactionRepository(oracleDB, log)

	downstreamClient := downstream.NewDownstreamClient(viperConfig, log)

	transactionUseCase := usecase.NewTransactionUseCase(
		transactionRepo, downstreamClient, validate, log,
	)

	handler := &consumerHandler{useCase: transactionUseCase}

	consumer := messaging.NewConsumer(
		rabbitMQ,
		handler,
		viperConfig,
		log,
		oracleDB,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queueFilter := viperConfig.GetString("rabbitmq.consumer.queue_filter")
	var queueNames []string
	if queueFilter == "*" {
		log.Info().Msg("Consumer configured to consume all queues")
		queueNames = []string{}
	} else {
		queueNames = strings.Split(queueFilter, ",")
		for i := range queueNames {
			queueNames[i] = strings.TrimSpace(queueNames[i])
		}
	}

	if len(queueNames) > 0 {
		consumer.Start(ctx, queueNames)
		log.Info().Msgf("Consumer started for queues: %v", queueNames)
	} else {
		log.Warn().Msg("No queue names configured, consumer idle")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info().Msgf("Received signal: %v. Shutting down worker...", sig)

	cancel()

	done := make(chan struct{})
	go func() {
		consumer.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("All workers stopped")
	case <-time.After(30 * time.Second):
		log.Warn().Msg("Shutdown timeout, forcing exit")
	}

	rabbitMQ.Close()
	config.CloseOracleDB(oracleDB, log)

	log.Info().Msg("Worker shutdown completed")
	os.Exit(0)
}

type consumerHandler struct {
	useCase *usecase.TransactionUseCase
}

func (h *consumerHandler) Handle(ctx context.Context, event *model.TransactionEvent) error {
	return h.useCase.HandleConsumedMessage(ctx, event)
}
