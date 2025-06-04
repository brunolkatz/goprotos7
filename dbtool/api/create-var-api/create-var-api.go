package create_var_api

import (
	"context"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type varsHandler interface {
	CreateVariable(ctx context.Context, newVar *dbtool.CreateVarRequest) (*db_models.DbVariables, error)
}

type CreateVarAPi struct {
	varsHandler varsHandler
}

func New() (*CreateVarAPi, error) {
	return &CreateVarAPi{}, nil
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
		case "int-field[]": // We will assume that exists the desc-int-field[] and default-int-value as well
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
			if _, ok := r.Form["desc-int-field[]"]; !ok {
				wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "desc-int-field[] not provided", w, r)
				return
			}
			for ivIdx, iv := range v {
				intVal, err := strconv.ParseInt(iv, 10, 64)
				if err != nil {
					wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Invalid int value provided: "+err.Error(), w, r)
					return
				}
				if req.IntFields == nil {
					req.IntFields = make([]*dbtool.IntFields, 0)
				}
				req.IntFields = append(req.IntFields, &dbtool.IntFields{
					Description: r.Form["desc-int-field[]"][ivIdx],
					Value:       intVal,
				})
			}

		}
	}

	wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Success, "Variable created successfully", w, r)
}
