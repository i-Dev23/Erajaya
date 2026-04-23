package database

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"pps-services-consumer/constanta"
	"pps-services-consumer/util"
	log "pps-services-consumer/util"
)

var (
	dbPool    *sql.DB
	poolMutex sync.Mutex
)

func GetEnvInt(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		log.Printf("Peringatan: ENV %s bukan angka valid (%v), pakai default %d", key, err, defaultValue)
		return defaultValue
	}
	return val
}

func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	// Bisa parse format seperti "30s", "5m", "1h"
	val, err := time.ParseDuration(valStr)
	if err != nil {
		log.Printf("Peringatan: ENV %s bukan durasi valid (%v), pakai default %v", key, err, defaultValue)
		return defaultValue
	}
	return val
}

// InitDBPool menginisialisasi connection pool.
func InitDBPool() (*sql.DB, error) {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if dbPool != nil {
		return dbPool, nil
	}

	dataSourceName := os.Getenv(constanta.OS_ENV_URL_CONNECTION)
	maxOpenConns := GetEnvInt(constanta.OS_ENV_DB_POOL_MAX_OPEN_CONNS, 10)
	maxIdleConns := GetEnvInt(constanta.OS_ENV_DB_POOL_MAX_IDLE_CONNS, 5)
	connMaxLifetime := GetEnvDuration(constanta.OS_ENV_DB_POOL_CONN_MAX_LIFETIME, 5*time.Minute)
	connMaxIdleTime := GetEnvDuration(constanta.OS_ENV_DB_POOL_CONN_MAX_IDLE_TIME, 2*time.Minute)

	if maxOpenConns == 0 {
		maxOpenConns = 25
	}
	if maxIdleConns == 0 {
		maxIdleConns = 10
	}

	db, err := sql.Open("oracle", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %v", err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	dbPool = db
	log.Printf("Database connection pool initialized - MaxOpen: %d, MaxIdle: %d, MaxLifetime: %v, MaxIdleTime: %v",
		maxOpenConns, maxIdleConns, connMaxLifetime, connMaxIdleTime)

	return dbPool, nil
}

// GetConnection mendapatkan koneksi dari pool, reconnect jika pool nil.
func GetConnection() (*sql.DB, error) {
	poolMutex.Lock()
	pool := dbPool
	poolMutex.Unlock()

	if pool != nil {
		return pool, nil
	}

	// Pool nil, coba reconnect
	util.ComposeMessageTelegramNotification("Database pool is nil, attempting to reinitialize")
	return InitDBPool()
}

// ResetPool menutup pool dan set nil agar bisa di-reinit oleh GetConnection.
func ResetPool() {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if dbPool != nil {
		dbPool.Close()
		dbPool = nil
		log.Println("Database pool reset")
	}
}

// ClosePool menutup semua koneksi dalam pool
func ClosePool() {
	if dbPool != nil {
		dbPool.Close()
		dbPool = nil
	}
}
