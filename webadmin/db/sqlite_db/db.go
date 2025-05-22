package sql_lite_db

import (
	"context"
	"fmt"
	"github.com/brunolkatz/goprotos7/webadmin"
	"github.com/charmbracelet/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is a database connection handler
type DB struct {
	baseCtx context.Context
	DbConn  *gorm.DB
	DSN     string
	log     *log.Logger
}

// New creates and returns a database connection
func New(ctx context.Context, dsn string, log *log.Logger) (*DB, error) {

	db := &DB{
		baseCtx: ctx,
		DSN:     dsn,
		log:     log,
	}
	err := db.SetDbConn(dsn)
	if err != nil {
		log.Errorf("(New) - Error setting db connection: %+v", err)
		return nil, err
	}

	err = db.runMigrations(ctx)
	if err != nil {
		log.Errorf("(New) - Error running migrations: %+v", err)
		return nil, webadmin.ErrSQLiteMigrationFailed
	}

	return db, nil
}

// SetDbConn runs the migration against the provided DSN
func (d *DB) SetDbConn(dsn string) error {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_auth&_auth_user=admin&_auth_pass=admin&_auth_crypt=sha1", dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return err
	}

	d.DbConn = db
	return nil
}

func (d *DB) IsConnected() bool {
	if d.DbConn == nil {
		return false
	}
	return true
}
