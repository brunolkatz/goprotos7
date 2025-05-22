package wa_server_templs

import (
	"github.com/a-h/templ"
	"net/http"
)

func RenderAlertMSG(returnType ReturnType, msg string, w http.ResponseWriter, r *http.Request) {
	// Render error page
	var comp templ.Component
	switch returnType {
	case RT_None:
		comp = InfoAlert(msg)
	case RT_Success:
		comp = SuccessAlert(msg)
	case RT_Error:
		comp = ErrorAlert(msg)
	default:
		comp = InfoAlert("Unknown error")
	}
	err := comp.Render(r.Context(), w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
