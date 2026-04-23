package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"pps-services-consumer/constanta"
	"pps-services-consumer/database"
	"pps-services-consumer/repository"
	"pps-services-consumer/util"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	amqp "github.com/rabbitmq/amqp091-go"
)

// queueConfig menyimpan konfigurasi satu pair queue + MQ URL.
type queueConfig struct {
	QueueName      string
	MQTransactionURL string
}

func main() {
	// Inisialisasi Oracle connection pool
	if _, err := database.InitDBPool(); err != nil {
		log.Fatalf("Failed to initialize database pool: %v", err)
	}
	log.Println("Successfully initialized database pool")
	defer database.ClosePool()

	// Parse queue configs dari env var
	queues := loadQueueConfigs()
	if len(queues) == 0 {
		log.Fatal("No queue configurations found. Set QUEUE_NAME_1 + MQ_TRANSACTION_1, etc.")
	}

	log.Printf("Loaded %d queue configuration(s)", len(queues))
	for i, q := range queues {
		log.Printf("  [%d] queue=%s, mq=%s", i+1, q.QueueName, q.MQTransactionURL)
	}

	// Start HTTP server di goroutine
	go route(os.Getenv(constanta.OS_ENV_PORT))

	// Graceful shutdown via signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consumer per queue config, masing-masing dengan auto-reconnect
	stopCh := make(chan struct{})
	for _, q := range queues {
		go runConsumerWithReconnect(q, stopCh)
	}

	// Block sampai signal shutdown
	sig := <-sigChan
	log.Printf("Received signal: %v. Shutting down...", sig)
	close(stopCh)

	// Beri waktu consumer untuk cleanup
	time.Sleep(2 * time.Second)
	os.Exit(0)
}

// loadQueueConfigs membaca pasangan env var QUEUE_NAME_{N} + MQ_TRANSACTION_{N}.
// Sequence dimulai dari 1. Berhenti jika QUEUE_NAME_{N} tidak ditemukan.
func loadQueueConfigs() []queueConfig {
	var configs []queueConfig
	for i := 1; ; i++ {
		queueName := os.Getenv(fmt.Sprintf("QUEUE_NAME_%d", i))
		mqURL := os.Getenv(fmt.Sprintf("MQ_TRANSACTION_%d", i))

		if queueName == "" {
			break // tidak ada lagi
		}
		if mqURL == "" {
			log.Printf("WARNING: QUEUE_NAME_%d=%s tapi MQ_TRANSACTION_%d kosong, skip", i, queueName, i)
			continue
		}

		configs = append(configs, queueConfig{
			QueueName:        queueName,
			MQTransactionURL: mqURL,
		})
	}
	return configs
}

// runConsumerWithReconnect menjalankan RabbitMQ consumer dengan auto-reconnect.
// Setiap queue config punya connection sendiri ke RabbitMQ.
// Jika connection drop, retry dengan exponential backoff (max 60s).
func runConsumerWithReconnect(cfg queueConfig, stopCh <-chan struct{}) {
	backoff := time.Second * 2
	maxBackoff := time.Second * 60
	tag := fmt.Sprintf("[%s]", cfg.QueueName)

	for {
		select {
		case <-stopCh:
			log.Printf("%s consumer stopped by shutdown signal", tag)
			return
		default:
		}

		conn, err := amqp.Dial(cfg.MQTransactionURL)
		if err != nil {
			errMsg := fmt.Sprintf("%s RabbitMQ connect failed: %s", tag, err.Error())
			util.ComposeMessageTelegramNotification(errMsg)
			log.Printf("%s, retry in %v", errMsg, backoff)

			select {
			case <-time.After(backoff):
			case <-stopCh:
				return
			}

			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = time.Second * 2
		log.Printf("%s connected to RabbitMQ", tag)

		closeCh := make(chan *amqp.Error, 1)
		conn.NotifyClose(closeCh)

		doneCh := make(chan struct{})
		go func() {
			repository.ConsumerFIFO(conn, cfg.QueueName, cfg.MQTransactionURL)
			close(doneCh)
		}()

		select {
		case amqpErr := <-closeCh:
			if amqpErr != nil {
				errMsg := fmt.Sprintf("%s RabbitMQ connection lost: %s", tag, amqpErr.Error())
				util.ComposeMessageTelegramNotification(errMsg)
				log.Println(errMsg)
			} else {
				log.Printf("%s RabbitMQ connection closed gracefully", tag)
			}
		case <-doneCh:
			log.Printf("%s consumer exited, will reconnect", tag)
		case <-stopCh:
			log.Printf("%s shutdown signal, closing connection", tag)
			conn.Close()
			return
		}

		conn.Close()
		log.Printf("%s reconnecting in %v...", tag, backoff)

		select {
		case <-time.After(backoff):
		case <-stopCh:
			return
		}
	}
}

func route(port string) {
	app := fiber.New()

	app.Use(cors.New())
	app.Use(logger.New())

	app.Get("/metrics", monitor.New(monitor.Config{Title: os.Getenv(constanta.OS_ENV_Env)}))

	log.Fatal(app.Listen(":" + port))
}
