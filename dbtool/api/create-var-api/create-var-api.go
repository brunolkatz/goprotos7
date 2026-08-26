package create_var_api

import (
	"context"
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"math"
	"net/http"
	"strconv"
	"strings"
)

type varsHandler interface {
	CreateVariable(ctx context.Context, newVar *dbtool.CreateVarRequest) (*db_models.DbVariable, error)
}

type CreateVarAPi struct {
	varsHandler varsHandler
}

func New(varsHandler varsHandler) (*CreateVarAPi, error) {
	if varsHandler == nil {
		return nil, fmt.Errorf("varsHandler is nil")
	}
	return &CreateVarAPi{
		varsHandler,
	}, nil
}

func (h *CreateVarAPi) Register(r chi.Router) {
	r.Route("/vars", func(r chi.Router) {
		r.Get("/", h.GetExamplePage)
		r.Get("/var-def", h.GetVarDefTempl)
		r.Post("/create-var", h.CreateNewVar)
	})
}

// GetExamplePage - Renders the main example page, loading all necessary first things....
func (h *CreateVarAPi) GetExamplePage(w http.ResponseWriter, r *http.Request) {

	err := wa_server_templs.RenderPageLayout(
		w,
		r,
		"Example Page",
		CreateVarPageTempl(),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (h *CreateVarAPi) GetVarDefTempl(w http.ResponseWriter, r *http.Request) {

	// Get db-select query from the request
	_varType := r.URL.Query().Get("data_type")
	if _varType == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("no variable type provided"))
		return
	}

	var varType goprotos7.DataType
	if vt, err := strconv.Atoi(_varType); err == nil {
		varType = goprotos7.DataType(uint32(vt))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("invalid variable type provided"))
		return
	}

	comp := CreateVarDefTempl(varType)
	if comp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("invalid variable type provided"))
		return
	}
	err := comp.Render(r.Context(), w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error rendering template: " + err.Error()))
		return
	}
}

func (h *CreateVarAPi) CreateNewVar(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm() // 10 MB
	if err != nil {
		wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Error parsing form: "+err.Error(), w, r)
		return
	}

	req := &dbtool.CreateVarRequest{
		Name:        "",
		Description: "",
		DataType:    goprotos7.DT_UNSED,
	}

	for field, v := range r.Form {
		switch field {
		case "db-number":
			intVal, err := strconv.ParseInt(r.Form.Get("db-number"), 10, 64)
			if err != nil {
				wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid string length provided: "+err.Error(), w, r)
				return
			}
			req.DBNumber = intVal
		case "name":
			req.Name = v[0]
		case "description":
			req.Description = v[0]
		case "data_type":
			dataType, err := strconv.ParseUint(v[0], 10, 32)
			if err != nil {
				wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid data type provided: "+err.Error(), w, r)
				return
			}
			req.DataType = goprotos7.DataType(dataType)
			switch req.DataType {
			case goprotos7.BOOL:
				if _, ok := r.Form["desc-bool-field[]"]; !ok {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "desc-bool-field[] not provided", w, r)
					return
				}

				for iIdx, desc := range r.Form["desc-bool-field[]"] {
					b := false
					switch strings.ToUpper(r.Form["bool-value[]"][iIdx]) {
					case "TRUE", "1":
						b = true
					case "FALSE", "0":
						b = false
					}
					req.ListFields = append(req.ListFields, &dbtool.ListFields{
						Description: desc,
						BoolValue:   &b,
						BitOffset:   getFormInt(r.Form["bit-bool-field[]"][iIdx]),
						StaticType:  dbtool.StaticTypeBool,
					})
				}
			case goprotos7.STRING, goprotos7.CHAR:
				intVal, err := strconv.ParseInt(r.Form.Get("str-length"), 10, 64)
				if err != nil {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid string length provided: "+err.Error(), w, r)
					return
				}
				if intVal > math.MaxUint8 {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "String length cannot be greater than 255", w, r)
					return
				} else if intVal < 0 {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "String length cannot be negative", w, r)
					return
				}
				u8 := uint8(intVal)
				req.StrLength = &u8

				if intVal > 1 && req.DataType == goprotos7.CHAR {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "CHAR data type cannot have length greater than 1", w, r)
					return
				}

				if defVal, ok := r.Form["str-default-value"]; ok {
					if len(defVal) > 0 {
						if len(defVal) > 1 && req.DataType == goprotos7.CHAR {
							wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "CHAR default value cannot have length greater than 1", w, r)
							return
						}
						req.StringVal = &defVal[0]
					} else {
						wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "str-default-value not provided", w, r)
						return
					}
				} else {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "str-default-value not provided", w, r)
					return
				}
			case goprotos7.SINT, goprotos7.USINT, goprotos7.INT, goprotos7.UINT, goprotos7.DINT, goprotos7.UDINT, goprotos7.LINT, goprotos7.ULINT:
				// Get the Default int value
				if iv, ok := r.Form["default-int-value"]; ok {
					intVal, err := strconv.ParseInt(iv[0], 10, 64)
					if err != nil {
						wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid default int value provided: "+err.Error(), w, r)
						return
					}
					req.IntVal = &intVal
				} else {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "default-int-value not provided", w, r)
					return
				}
				// Verify if the desc-int-field[] and int-field[] exists
				if _, ok := r.Form["desc-int-field[]"]; !ok {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "desc-int-field[] not provided", w, r)
					return
				}
				if intFields, ok := r.Form["int-field[]"]; !ok {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "int-field[] not provided", w, r)
					return
				} else {
					for ivIdx, iv := range intFields {
						intVal, err := strconv.ParseInt(iv, 10, 64)
						if err != nil {
							wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid int value provided: "+err.Error(), w, r)
							return
						}
						if req.ListFields == nil {
							req.ListFields = make([]*dbtool.ListFields, 0)
						}
						req.ListFields = append(req.ListFields, &dbtool.ListFields{
							Description: r.Form["desc-int-field[]"][ivIdx],
							IntValue:    &intVal,
						})
					}
				}
			default:
			}
		}
	}

	_, err = h.varsHandler.CreateVariable(r.Context(), req)
	if err != nil {
		wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Error creating variable: "+err.Error(), w, r)
		return
	}
	wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Success, "Variable created successfully", w, r)
}

// getFormInt - Helper function to get an int value from the form, panics if the value is not a valid int.
func getFormInt(v string) *int64 {
	r, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic("go off")
	}
	return &r
}

func GetFormFloat(v string) *float64 {
	r, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic("go off")
	}
	return &r
}
