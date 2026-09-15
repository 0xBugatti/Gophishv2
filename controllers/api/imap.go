package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/imap"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// IMAPServerValidate handles requests for the /api/imapserver/validate endpoint
func (as *Server) IMAPServerValidate(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		JSONResponse(w, models.Response{Success: false, Message: "Only POSTs allowed"}, http.StatusBadRequest)
	case r.Method == "POST":
		im := models.IMAP{}
		err := json.NewDecoder(r.Body).Decode(&im)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		err = imap.Validate(&im)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusOK)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Successful login."}, http.StatusCreated)
	}
}

// IMAPServer handles requests for the /api/imapserver/ endpoint
func (as *Server) IMAPServer(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ss, err := models.GetIMAP(ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, ss, http.StatusOK)

	// POST: Create new IMAP configuration
	case r.Method == "POST":
		im := models.IMAP{}
		err := json.NewDecoder(r.Body).Decode(&im)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid data. Please check your IMAP settings."}, http.StatusBadRequest)
			return
		}
		im.ModifiedDate = time.Now().UTC()
		im.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostIMAP(&im, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Successfully created new IMAP configuration.", Data: im}, http.StatusCreated)
	}
}

// nonCampaignReportsResponse is the JSON response for non-campaign report listings
type nonCampaignReportsResponse struct {
	Stats          models.NonCampaignStats      `json:"stats"`
	IMAPConfigs    []imapConfigSummary          `json:"imap_configs"`
	SelectedIMAPId int64                        `json:"selected_imap_id"`
	Reports        []models.NonCampaignReport   `json:"reports"`
}

type imapConfigSummary struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type bulkDeleteRequest struct {
	Ids []int64 `json:"ids"`
}

// IMAPNonCampaignReports handles GET/POST/DELETE for /api/imap/non_campaign_reports
func (as *Server) IMAPNonCampaignReports(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)

	switch r.Method {
	case "GET":
		// Parse optional imap_id filter from query string
		var imapId int64
		if idStr := r.URL.Query().Get("imap_id"); idStr != "" {
			imapId, _ = strconv.ParseInt(idStr, 10, 64)
		}

		stats, err := models.GetNonCampaignStats(uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		// Fetch IMAP configs owned by user for filter dropdown
		imapList, err := models.GetIMAP(uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		configs := make([]imapConfigSummary, 0, len(imapList))
		for _, im := range imapList {
			configs = append(configs, imapConfigSummary{Id: im.Id, Name: im.Name})
		}

		reports, err := models.GetNonCampaignReports(uid, imapId)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		resp := nonCampaignReportsResponse{
			Stats:          stats,
			IMAPConfigs:    configs,
			SelectedIMAPId: imapId,
			Reports:        reports,
		}
		JSONResponse(w, resp, http.StatusOK)

	case "POST":
		// Bulk delete by IDs
		req := bulkDeleteRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || len(req.Ids) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request: provide ids array"}, http.StatusBadRequest)
			return
		}
		err = models.BulkDeleteNonCampaignReports(uid, req.Ids)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Reports deleted successfully."}, http.StatusOK)

	case "DELETE":
		// Delete all reports, optionally filtered by imap_id
		var imapId int64
		if idStr := r.URL.Query().Get("imap_id"); idStr != "" {
			imapId, _ = strconv.ParseInt(idStr, 10, 64)
		}
		var err error
		if imapId > 0 {
			err = models.DeleteNonCampaignReportsByImapId(uid, imapId)
		} else {
			err = models.DeleteAllNonCampaignReports(uid)
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Reports deleted successfully."}, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// IMAPServerById handles requests for the /api/imapserver/:id endpoint
func (as *Server) IMAPServerById(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid IMAP configuration ID."}, http.StatusBadRequest)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)

	switch {
	case r.Method == "GET":
		im, err := models.GetIMAPById(id, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "IMAP configuration not found."}, http.StatusNotFound)
			return
		}
		JSONResponse(w, im, http.StatusOK)

	case r.Method == "PUT":
		im := models.IMAP{}
		err := json.NewDecoder(r.Body).Decode(&im)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid data. Please check your IMAP settings."}, http.StatusBadRequest)
			return
		}
		im.Id = id
		im.ModifiedDate = time.Now().UTC()
		err = models.UpdateIMAP(&im, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		// Log clear messages about configuration changes
		if im.Enabled {
			log.Infof("IMAP configuration ID %d enabled for user ID %d", im.Id, uid)
		} else {
			log.Infof("IMAP configuration ID %d disabled for user ID %d", im.Id, uid)
		}

		JSONResponse(w, models.Response{Success: true, Message: "Successfully updated IMAP configuration."}, http.StatusOK)

	case r.Method == "DELETE":
		err := models.DeleteIMAPById(id, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "IMAP configuration deleted successfully."}, http.StatusOK)
	}
}
