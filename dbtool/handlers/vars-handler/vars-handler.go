package vars_handler

import (
	"context"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	sql_lite_db "github.com/brunolkatz/goprotos7/dbtool/db/sqlite_db"
	"github.com/charmbracelet/log"
)

type WebAdminHandler struct {
	db *sql_lite_db.DB
}

func New(db *sql_lite_db.DB, log *log.Logger) (*WebAdminHandler, error) {
	return &WebAdminHandler{
		db: db,
	}, nil
}

func (h *WebAdminHandler) CreateVariable(ctx context.Context, newVar *dbtool.CreateVarRequest) (*db_models.DbVariables, error) {
	return nil, fmt.Errorf("implemet me please")
}
