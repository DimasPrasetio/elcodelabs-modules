// Package database opens the connection pool to the host's shared MySQL.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Open returns the pool without dialing: the API starts even while MySQL is down, and
// /readyz reports it until it answers — deploy.sh and the image HEALTHCHECK wait on that.
func Open(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DB_DSN: %w", err)
	}
	// sqlc scans DATETIME/TIMESTAMP columns into time.Time, which needs parseTime. Set here
	// rather than in DB_DSN, because golang-migrate reads the same variable.
	cfg.ParseTime = true
	// Every session runs in UTC whatever the host MySQL's time zone is, matching the driver's
	// default Loc (UTC). Without it, TIMESTAMP columns written and read through a server set
	// to WIB shift by seven hours. Day boundaries ("today" for a cashier) are computed in
	// Asia/Jakarta explicitly, in code.
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// A small shared VPS: keep the pool modest, and recycle connections well before
	// MySQL's wait_timeout closes them from the other side.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

// Name is the database DB_DSN points at — what names this app's MySQL locks on a shared server.
func Name(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return ""
	}
	return cfg.DBName
}

// Ping reports whether the database answers within timeout.
func Ping(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(ctx)
}
