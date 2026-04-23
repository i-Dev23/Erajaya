package repository

import (
	"github.com/rs/zerolog"
)

type TransactionRepository struct {
	log zerolog.Logger
}

func NewTransactionRepository(log zerolog.Logger) *TransactionRepository {
	return &TransactionRepository{
		log: log,
	}
}
