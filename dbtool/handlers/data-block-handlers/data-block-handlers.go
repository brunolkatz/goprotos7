package data_block_handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/db/sqlite_db"
	build_datab "github.com/brunolkatz/goprotos7/dbtool/internals/build-datab"
	"github.com/charmbracelet/log"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

type DBBlocksHandler struct {
	baseCtx     context.Context
	defaultPath string
	sqlite      *sql_lite_db.DB
	log         *log.Logger
}

func New(baseCtx context.Context, defaultPath string, sqlite *sql_lite_db.DB, log *log.Logger) (*DBBlocksHandler, error) {
	if log == nil {
		return nil, errors.New("[DB_BLOCKS_HANDLER] logger is nil")
	}
	if defaultPath == "" {
		defaultPath = os.Getenv("DB_BIN_PATH")
		if defaultPath == "" { // Set to pwd
			var err error
			defaultPath, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("[DB_BLOCKS_HANDLER] error getting current working directory: %w", err)
			}
		}
	}
	return &DBBlocksHandler{
		baseCtx:     baseCtx,
		defaultPath: defaultPath,
		sqlite:      sqlite,
		log:         log,
	}, nil
}

// CreateDatabaseBlocks - Create/Recreate database blocks for all DBs in the SQLite database.
func (h *DBBlocksHandler) CreateDatabaseBlocks() error {
	dbNumbers, err := h.sqlite.GetDBNumbers(h.baseCtx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if len(dbNumbers) == 0 {
		return nil
	}
	for _, dbNumber := range dbNumbers {
		var dbVariables []*db_models.DbVariable
		dbVariables, err = h.sqlite.GetDbVariables_ASC(h.baseCtx, dbNumber)
		if err != nil {
			return err
		}
		if h.defaultPath == "" {
			h.defaultPath, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		err = build_datab.BuildDataBlocks(filepath.Join(h.defaultPath, fmt.Sprintf("DB%d.bin", dbNumber)), dbVariables)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *DBBlocksHandler) WriteToDb(_ context.Context, dbVar *db_models.DbVariable) error {
	if dbVar == nil {
		return errors.New("dbVar is nil")
	}
	if h.defaultPath == "" {
		h.defaultPath, _ = os.Getwd()
	}
	_, err := build_datab.WriteVariableToFile(filepath.Join(h.defaultPath, fmt.Sprintf("DB%d.bin", dbVar.DbNumber)), dbVar)
	if err != nil {
		h.log.Errorf("Error writing variable to file: %v", err)
		return fmt.Errorf("error writing variable to file: %w", err)
	}
	return nil
}
