package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"codeatlas/packages/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresConfig struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	ConnectTimeout  time.Duration
	HealthcheckFreq time.Duration
}

func LoadPostgresConfigFromEnv() (PostgresConfig, error) {
	port, err := config.GetInt("POSTGRES_PORT", 5432)
	if err != nil {
		return PostgresConfig{}, err
	}

	maxConns, err := config.GetInt("POSTGRES_MAX_CONNS", 10)
	if err != nil {
		return PostgresConfig{}, err
	}

	minConns, err := config.GetInt("POSTGRES_MIN_CONNS", 2)
	if err != nil {
		return PostgresConfig{}, err
	}

	connectTimeout, err := config.GetDuration("POSTGRES_CONNECT_TIMEOUT", 5*time.Second)
	if err != nil {
		return PostgresConfig{}, err
	}

	healthcheckFreq, err := config.GetDuration("POSTGRES_HEALTHCHECK_FREQ", 30*time.Second)
	if err != nil {
		return PostgresConfig{}, err
	}

	user, err := config.MustString("POSTGRES_USER")
	if err != nil {
		return PostgresConfig{}, err
	}

	password, err := config.MustString("POSTGRES_PASSWORD")
	if err != nil {
		return PostgresConfig{}, err
	}

	databaseName, err := config.MustString("POSTGRES_DB")
	if err != nil {
		return PostgresConfig{}, err
	}

	return PostgresConfig{
		Host:            config.GetString("POSTGRES_HOST", "localhost"),
		Port:            port,
		Database:        databaseName,
		User:            user,
		Password:        password,
		SSLMode:         config.GetString("POSTGRES_SSL_MODE", "disable"),
		MaxConns:        int32(maxConns),
		MinConns:        int32(minConns),
		ConnectTimeout:  connectTimeout,
		HealthcheckFreq: healthcheckFreq,
	}, nil
}

func (c PostgresConfig) ConnectionString() string {
	query := url.Values{}
	query.Set("sslmode", c.SSLMode)
	query.Set("connect_timeout", fmt.Sprintf("%.0f", c.ConnectTimeout.Seconds()))

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?%s",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		c.Database,
		query.Encode(),
	)
}

func NewPostgresPool(ctx context.Context, cfg PostgresConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.HealthCheckPeriod = cfg.HealthcheckFreq

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
