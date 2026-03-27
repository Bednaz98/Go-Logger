package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const sqliteDriverRegexp = "sqlite3_with_regexp"

func init() {
	sql.Register(sqliteDriverRegexp, &sqlite3.SQLiteDriver{
		ConnectHook: func(c *sqlite3.SQLiteConn) error {
			return c.RegisterFunc("regexp", func(re, s string) (interface{}, error) {
				matched, err := regexp.MatchString(re, s)
				if err != nil {
					return false, err
				}
				return matched, nil
			}, true)
		},
	})
}

// OpenDB opens GORM for Postgres (postgres:// or DATABASE_URL) or SQLite file / in-memory.
func OpenDB(dsn string, logSQL bool) (*gorm.DB, string, error) {
	cfg := &gorm.Config{}
	if !logSQL {
		cfg.Logger = logger.Default.LogMode(logger.Silent)
	}

	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, "", fmt.Errorf("store: empty database dsn")
	}

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		db, err := gorm.Open(postgres.Open(dsn), cfg)
		if err != nil {
			return nil, "", err
		}
		return db, "postgres", nil
	}

	// SQLite: file path or sqlite://path or :memory:
	sqliteDSN := dsn
	if strings.HasPrefix(strings.ToLower(dsn), "sqlite://") {
		sqliteDSN = strings.TrimPrefix(dsn, "sqlite://")
		sqliteDSN = strings.TrimPrefix(sqliteDSN, "SQLite://")
	}
	dialector := sqlite.Dialector{
		DriverName: sqliteDriverRegexp,
		DSN:        sqliteDSN,
	}
	db, err := gorm.Open(dialector, cfg)
	if err != nil {
		return nil, "", err
	}
	return db, "sqlite", nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Log{})
}
