package data_block_handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/brunolkatz/goprotos7/webadmin/db/db_models"
	"github.com/brunolkatz/goprotos7/webadmin/db/sqlite_db"
	build_datab "github.com/brunolkatz/goprotos7/webadmin/internals/build-datab"
	"github.com/charmbracelet/log"
	"os"
	"path/filepath"
)

type DBBlocksHandler struct {
	baseCtx context.Context
	sqlite  *sql_lite_db.DB
	log     *log.Logger
}

func New(baseCtx context.Context, sqlite *sql_lite_db.DB, log *log.Logger) (*DBBlocksHandler, error) {
	if log == nil {
		return nil, errors.New("[DB_BLOCKS_HANDLER] logger is nil")
	}
	return &DBBlocksHandler{
		baseCtx: baseCtx,
		sqlite:  sqlite,
		log:     log,
	}, nil
}

func (h *DBBlocksHandler) CreateDatabaseBlocks(path string) error {
	dbNumbers, err := h.sqlite.GetDBNumbers(h.baseCtx)
	if err != nil {
		return err
	}
	if len(dbNumbers) == 0 {
		return errors.New("[DB_BLOCKS_HANDLER] no database blocks found, SQLite is empty")
	}
	for _, dbNumber := range dbNumbers {
		var dbVariables []*db_models.DbVariables
		dbVariables, err = h.sqlite.GetDbVariables(h.baseCtx, dbNumber)
		if err != nil {
			return err
		}
		if path == "" {
			path, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		err = build_datab.BuildDataBlocks(filepath.Join(path, fmt.Sprintf("DB%d.bin", dbNumber)), dbVariables)
		if err != nil {
			return err
		}
	}
	return nil
}
