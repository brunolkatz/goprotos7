package dashboard_api

import (
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool"
	"net/http"
	"strconv"
)

type getDbVarsQuery struct {
	DBNumber int32
}

func bindGetDbVarsQuery(r *http.Request) (*getDbVarsQuery, error) {
	raw := r.URL.Query().Get("db-number")
	if raw == "" {
		return nil, fmt.Errorf("dbNumber is required")
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid dbNumber: %w", err)
	}
	return &getDbVarsQuery{DBNumber: int32(n)}, nil
}

type setDbVarForm struct {
	DBNumber int64
	VarID    int64
	StatusID int64
	VarType  dbtool.VarType
}

func bindSetDbVarForm(r *http.Request) (*setDbVarForm, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("error parsing form: %w", err)
	}

	rawDBNumber := r.Form.Get("db-number")
	if rawDBNumber == "" {
		return nil, fmt.Errorf("dbNumber is required")
	}
	dbNumber, err := strconv.ParseInt(rawDBNumber, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dbNumber: %w", err)
	}

	rawVarID := r.Form.Get("var-id")
	if rawVarID == "" {
		return nil, fmt.Errorf("varId is required")
	}
	varID, err := strconv.ParseInt(rawVarID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid varId: %w", err)
	}

	rawVarType := r.Form.Get("t")
	if rawVarType == "" {
		return nil, fmt.Errorf("variable type (t) is required")
	}
	varType, ok := dbtool.VarTypePara[rawVarType]
	if !ok {
		return nil, fmt.Errorf("invalid variable type: %s", rawVarType)
	}

	rawStatusID := r.Form.Get("sts-id")
	if rawStatusID == "" {
		return nil, fmt.Errorf("sts-id is required")
	}
	statusID, err := strconv.ParseInt(rawStatusID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid sts-id: %w", err)
	}

	return &setDbVarForm{
		DBNumber: dbNumber,
		VarID:    varID,
		StatusID: statusID,
		VarType:  varType,
	}, nil
}
