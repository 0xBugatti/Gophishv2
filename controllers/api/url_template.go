package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// URLTemplates handles requests for the /api/url_templates/ endpoint
func (as *Server) URLTemplates(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ts, err := models.GetURLTemplates(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ts, http.StatusOK)
	case r.Method == "DELETE":
		var req struct {
			IDs []int64 `json:"ids"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if len(req.IDs) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "No URL template IDs provided"}, http.StatusBadRequest)
			return
		}
		err = models.DeleteURLTemplates(req.IDs, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting URL templates: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "URL templates deleted successfully!"}, http.StatusOK)
	case r.Method == "POST":
		t := models.URLTemplate{}
		err := json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		_, err = models.GetURLTemplateByName(t.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "URL template name already in use"}, http.StatusConflict)
			log.Error(err)
			return
		}
		t.UserId = ctx.Get(r, "user_id").(int64)
		err = t.Validate()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		err = models.PostURLTemplate(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, t, http.StatusCreated)
	}
}

// URLTemplate handles requests for the /api/url_templates/:id endpoint
func (as *Server) URLTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	t, err := models.GetURLTemplate(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "URL template not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, t, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteURLTemplate(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting URL template"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "URL Template Deleted Successfully"}, http.StatusOK)
	case r.Method == "PUT":
		t = models.URLTemplate{}
		err = json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		if t.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and /:template_id mismatch"}, http.StatusBadRequest)
			return
		}
		t.UserId = ctx.Get(r, "user_id").(int64)
		err = t.Validate()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		err = models.PutURLTemplate(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating URL template"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, t, http.StatusOK)
	}
}
