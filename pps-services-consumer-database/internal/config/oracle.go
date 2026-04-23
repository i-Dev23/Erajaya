package config

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	_ "github.com/sijms/go-ora/v2"
	"github.com/spf13/viper"
)

// NewOracleDB creates an Oracle DB connection pool using go-ora (pure Go driver).
func NewOracleDB(config *viper.Viper, log zerolog.Logger) *sql.DB {
	host := config.GetString("oracle.host")
	port := config.GetInt("oracle.port")
	service := config.GetString("oracle.service")
	username := config.GetString("oracle.username")
	password := config.GetString("oracle.password")

	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		username, password, host, port, service)

	db, err := sql.Open("oracle", dsn)
	if err != nil {
		log.Fatal().Msgf("Failed to open Oracle connection: %v", err)
	}

	maxOpen := config.GetInt("oracle.pool.max_open")
	maxIdle := config.GetInt("oracle.pool.max_idle")
	maxLifetime := config.GetInt("oracle.pool.max_lifetime")

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Second)

	if err := db.Ping(); err != nil {
		log.Warn().Msgf("Oracle DB ping failed (will retry on demand): %v", err)
	} else {
		log.Info().Msgf("Successfully connected to Oracle DB at %s:%d/%s", host, port, service)
	}

	return db
}

// CloseOracleDB gracefully closes the Oracle DB connection.
func CloseOracleDB(db *sql.DB, log zerolog.Logger) {
	if db == nil {
		return
	}
	if err := db.Close(); err != nil {
		log.Error().Msgf("Failed to close Oracle DB: %v", err)
	} else {
		log.Info().Msg("Oracle DB connection closed")
	}
}
