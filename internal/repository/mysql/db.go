package mysql

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"

	"nehuang-iot-server/internal/config"
)

func Open(cfg config.MySQLConfig) (*sql.DB, error) {
	driverCfg := mysql.NewConfig()
	driverCfg.User = cfg.User
	driverCfg.Passwd = cfg.Password
	driverCfg.DBName = cfg.Database
	driverCfg.ParseTime = true
	driverCfg.Loc = time.Local
	driverCfg.Params = map[string]string{
		"charset": "utf8mb4",
	}

	if cfg.Socket != "" {
		driverCfg.Net = "unix"
		driverCfg.Addr = cfg.Socket
	} else {
		driverCfg.Net = "tcp"
		driverCfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}

	db, err := sql.Open("mysql", driverCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping() error {
	return r.db.Ping()
}
