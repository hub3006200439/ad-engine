package postgres

import (
	"context"
	"time"

	"ad-engine/internal/config"
	"ad-engine/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewDB создаёт и настраивает пул подключений к Postgres.
func NewDB(cfg config.PGStorageConfig) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, err
	}

	// Базовые настройки пула (при желании вынести в конфиг)
	conf.MaxConns = 50
	conf.MinConns = 5
	conf.MaxConnIdleTime = 5 * time.Minute
	conf.MaxConnLifetime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	logger.LogInfo("connected to postgres {host}:{port}/{db}", cfg.Host, cfg.Port, cfg.DBName)

	return pool, nil
}
