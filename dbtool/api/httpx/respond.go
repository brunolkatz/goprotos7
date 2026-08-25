package httpx

import (
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"net/http"
)

func BadRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func InternalError(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusInternalServerError)
}

func AlertError(w http.ResponseWriter, r *http.Request, msg string) {
	wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, msg, w, r)
}

func AlertSuccess(w http.ResponseWriter, r *http.Request, msg string) {
	wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Success, msg, w, r)
}
