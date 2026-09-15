package api

import (
	"net/http"

	"github.com/gophish/gophish/models"
)

// AuditLogs returns paginated audit log entries (6.12).
// GET /api/audit_log/?page=1&per_page=50
func (as *Server) AuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	page, perPage := paginationParams(r)
	logs, err := models.GetAuditLogs(page, perPage)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, logs, http.StatusOK)
}
