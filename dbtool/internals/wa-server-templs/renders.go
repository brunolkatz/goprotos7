package wa_server_templs

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/brunolkatz/goprotos7/dbtool"
	"net/http"
)

// RenderPageLayout renders the page layout with the given title and content.
// Uses the request header to load the page content only or the entire layout if not an HTMX request.
// mainly used the root subpage when user accesses the page directly from external link
func RenderPageLayout(w http.ResponseWriter, r *http.Request, title string, content templ.Component) error {
	if content == nil {
		return fmt.Errorf("content is nil")
	}
	isHtmxReq := dbtool.IsHTMXReq(r)
	if !isHtmxReq {
		var t *string
		if title != "" {
			s := dbtool.GetSubPageWebAdminTitle(title)
			t = &s
		}
		comp := PageLayout(t, content)
		err := comp.Render(r.Context(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		return nil
	} else {
		err := content.Render(r.Context(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		return nil
	}
}
