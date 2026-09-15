package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		// Apply pagination if ?page= is provided (9.3).
		page, perPage := paginationParams(r)
		start, end := paginateSlice(len(cs), page, perPage)
		JSONResponse(w, cs[start:end], http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		// Update editable campaign fields (4.2). Queued campaigns allow more
		// changes; running campaigns only allow name/description/dates.
		uc := models.Campaign{}
		if err := json.NewDecoder(r.Body).Decode(&uc); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		uc.Id = id
		if err := models.PutCampaign(&uc, ctx.Get(r, "user_id").(int64)); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// Return the full updated campaign
		updated, _ := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
		JSONResponse(w, updated, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
// Optional query param ?status=<status> filters by result status (6.7).
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		// Filter by status if ?status= query param is provided (6.7).
		if statusFilter := r.URL.Query().Get("status"); statusFilter != "" {
			filtered := cr.Results[:0]
			for _, res := range cr.Results {
				if strings.EqualFold(res.Status, statusFilter) {
					filtered = append(filtered, res)
				}
			}
			cr.Results = filtered
		}
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}

func (as *Server) FalsePositive(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	rid, _ := vars["rid"]
	switch {
	case r.Method == "GET":
		err := models.MarkEvent(id, rid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error marking event as false positive"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Event marked as false positive!"}, http.StatusOK)
	}
}

// ResendAll resends all the emails in a campaign.
func (as *Server) ResendAll(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		vars := mux.Vars(r)
		user := ctx.Get(r, "user").(models.User)
		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign ID"}, http.StatusBadRequest)
			return
		}

		_, err = models.GetCampaign(id, user.Id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found or access denied"}, http.StatusNotFound)
			return
		}

		err = models.ResendAllResults(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error queueing emails for resending"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Emails successfully queued for resending"}, http.StatusOK)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// CampaignPause pauses an in-progress campaign so no further emails are
// dispatched until it is resumed (4.1).
func (as *Server) CampaignPause(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	if err := models.PauseCampaign(id, uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Campaign paused successfully!"}, http.StatusOK)
}

// CampaignResume resumes a paused campaign (4.1).
func (as *Server) CampaignResume(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	if err := models.ResumeCampaign(id, uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Campaign resumed successfully!"}, http.StatusOK)
}

// CampaignResultsCSV streams campaign results as a CSV file (6.8).
func (as *Server) CampaignResultsCSV(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	cr, err := models.GetCampaignResults(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="campaign_results.csv"`)
	// UTF-8 BOM so Excel opens the file correctly
	w.Write([]byte("\xef\xbb\xbf"))
	fmt.Fprintf(w, "id,email,first_name,last_name,position,status,ip,latitude,longitude,send_date,reported,modified_date\r\n")
	for _, r := range cr.Results {
		fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s,%f,%f,%s,%t,%s\r\n",
			r.RId, csvEscape(r.Email), csvEscape(r.FirstName), csvEscape(r.LastName),
			csvEscape(r.Position), csvEscape(r.Status),
			csvEscape(r.IP), r.Latitude, r.Longitude,
			r.SendDate.UTC().Format("2006-01-02T15:04:05Z"),
			r.Reported,
			r.ModifiedDate.UTC().Format("2006-01-02T15:04:05Z"),
		)
	}
}

// csvEscape wraps a field in double quotes and escapes embedded double quotes
// for RFC 4180 compliance.
func csvEscape(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// CampaignResultReport manually marks a specific result as reported (6.6).
// POST /api/campaigns/{id}/results/{rid}/report
// Optional JSON body: {"reported_at": "2006-01-02T15:04:05Z"}
func (as *Server) CampaignResultReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rid := vars["rid"]
	uid := ctx.Get(r, "user_id").(int64)

	var payload struct {
		ReportedAt string `json:"reported_at"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	var reportedAt time.Time
	if payload.ReportedAt != "" {
		var err error
		reportedAt, err = time.Parse(time.RFC3339, payload.ReportedAt)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid reported_at timestamp"}, http.StatusBadRequest)
			return
		}
	}
	if err := models.MarkResultReported(rid, uid, reportedAt); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Result marked as reported"}, http.StatusOK)
}

// CampaignResultDeleteCredentials removes captured credential data from all
// Submitted Data events for the given result (6.5).
// DELETE /api/campaigns/{id}/results/{rid}/credentials
func (as *Server) CampaignResultDeleteCredentials(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rid := vars["rid"]
	uid := ctx.Get(r, "user_id").(int64)
	if err := models.DeleteResultCredentials(rid, uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Credentials deleted successfully"}, http.StatusOK)
}

// CampaignResultExclude removes an individual target from active campaign
// metrics and cancels any pending emails for them (4.4).
// DELETE /api/campaigns/{id}/results/{rid}
func (as *Server) CampaignResultExclude(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rid := vars["rid"]
	uid := ctx.Get(r, "user_id").(int64)
	if err := models.ExcludeResult(rid, uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Target excluded from campaign"}, http.StatusOK)
}

// CampaignResultsHistorical returns campaign results filtered by a time cutoff
// or time range (6.1). Supports two modes via query params:
//   ?mode=snapshot&cutoff=<RFC3339> — returns events that occurred before cutoff
//   ?mode=range&start=<RFC3339>&end=<RFC3339> — returns events in time range
func (as *Server) CampaignResultsHistorical(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	cr, err := models.GetCampaignResults(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	mode := r.URL.Query().Get("mode")
	switch mode {
	case "snapshot":
		cutoffStr := r.URL.Query().Get("cutoff")
		cutoff, err := time.Parse(time.RFC3339, cutoffStr)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid cutoff timestamp"}, http.StatusBadRequest)
			return
		}
		filtered := cr.Events[:0]
		for _, e := range cr.Events {
			if !e.Time.After(cutoff) {
				filtered = append(filtered, e)
			}
		}
		cr.Events = filtered
	case "range":
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")
		start, err1 := time.Parse(time.RFC3339, startStr)
		end, err2 := time.Parse(time.RFC3339, endStr)
		if err1 != nil || err2 != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid start/end timestamps"}, http.StatusBadRequest)
			return
		}
		filtered := cr.Events[:0]
		for _, e := range cr.Events {
			if !e.Time.Before(start) && !e.Time.After(end) {
				filtered = append(filtered, e)
			}
		}
		cr.Events = filtered
	default:
		JSONResponse(w, models.Response{Success: false, Message: "mode must be 'snapshot' or 'range'"}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, cr, http.StatusOK)
}

// AggregateResults returns merged results across multiple campaigns (6.3).
// GET /api/results/aggregate?campaign_ids=1,2,3
// AggregateResults returns merged results across multiple campaigns.
// Supports two modes:
//   - ?campaign_ids=1,2,3 — explicit campaign list (6.3)
//   - ?group_id=X — all campaigns that target the given group (6.2)
func (as *Server) AggregateResults(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)

	var campaignIds []int64
	if idsStr := r.URL.Query().Get("campaign_ids"); idsStr != "" {
		for _, p := range strings.Split(idsStr, ",") {
			cid, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err == nil {
				campaignIds = append(campaignIds, cid)
			}
		}
	} else if gidStr := r.URL.Query().Get("group_id"); gidStr != "" {
		gid, err := strconv.ParseInt(gidStr, 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "invalid group_id"}, http.StatusBadRequest)
			return
		}
		cids, err := models.GetCampaignIdsByGroupId(gid, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		campaignIds = cids
	} else {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids or group_id parameter required"}, http.StatusBadRequest)
		return
	}

	var allResults []models.Result
	var allEvents []models.Event
	for _, cid := range campaignIds {
		cr, err := models.GetCampaignResults(cid, uid)
		if err != nil {
			continue
		}
		allResults = append(allResults, cr.Results...)
		allEvents = append(allEvents, cr.Events...)
	}
	resp := models.CampaignResults{
		Id:      0,
		Name:    "Aggregate",
		Status:  "",
		Results: allResults,
		Events:  allEvents,
	}
	JSONResponse(w, resp, http.StatusOK)
}

// Resend resends a single email from a campaign.
func (as *Server) Resend(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		vars := mux.Vars(r)
		user := ctx.Get(r, "user").(models.User)
		rid := vars["rid"] // Get the string "rid" from the URL

		err := models.ResendResultByRId(rid, user.Id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Email successfully queued for resending"}, http.StatusOK)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// CampaignLink generates a generic tracking link for the given campaign.
// POST /api/campaigns/{id}/links
// Body (optional): {"name": "label for this link"}
// Returns the phishing URL with a pre-generated RId so the link can be
// embedded in documents, QR codes, or any non-email channel.
func (as *Server) CampaignLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 10, 64)
	uid := ctx.Get(r, "user_id").(int64)
	req := struct {
		Name string `json:"name"`
	}{}
	json.NewDecoder(r.Body).Decode(&req) // name is optional; ignore decode errors
	link, err := models.GenerateCampaignLink(id, uid, req.Name)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusNotFound)
		return
	}
	JSONResponse(w, link, http.StatusCreated)
}
