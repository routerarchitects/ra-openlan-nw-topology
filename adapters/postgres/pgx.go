package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/logger"
)

type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if log := logger.GetLogger(); log != nil {
		log.WithFields(logger.Fields{
			"component": "postgres.pool",
			"host":      cfg.Host,
			"port":      cfg.Port,
			"database":  cfg.Database,
			"max_conns": cfg.MaxConns,
			"min_conns": cfg.MinConns,
		}).Info("initializing postgres connection pool")
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "parse pgx config", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pcfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	p, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "create pgx pool", err)
	}
	if log := logger.GetLogger(); log != nil {
		log.WithFields(logger.Fields{
			"component": "postgres.pool",
			"host":      cfg.Host,
			"port":      cfg.Port,
			"database":  cfg.Database,
			"max_conns": cfg.MaxConns,
			"min_conns": cfg.MinConns,
		}).Info("postgres connection pool ready")
	}
	return p, nil
}
