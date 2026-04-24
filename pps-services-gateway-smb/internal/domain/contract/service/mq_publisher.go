package service

import "context"

// MQPublisher mendefinisikan kontrak untuk mempublikasikan pesan ke RabbitMQ.
// Setiap pemanggilan Publish membuka koneksi baru ke mqTransactionURL
// karena setiap transaksi dapat memiliki URL RabbitMQ yang berbeda.
type MQPublisher interface {
	// Publish mempublikasikan body ke queue queueName pada RabbitMQ di mqTransactionURL.
	Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error
}
