package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/gophish/gophish/logger"
)

// paginationParams parses optional ?page= and ?per_page= query params (9.3).
// Returns (page, perPage) with 1-based page index and capped perPage (max 1000).
// If no params are given, page=0 indicates no pagination requested.
func paginationParams(r *http.Request) (int, int) {
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")
	if pageStr == "" {
		return 0, 0
	}
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(perPageStr)
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 1000 {
		perPage = 1000
	}
	return page, perPage
}

// paginateSlice returns the sub-slice for the requested page. Returns the
// start and end indices. If page==0 (no pagination), returns (0, total).
func paginateSlice(total, page, perPage int) (int, int) {
	if page == 0 || total == 0 {
		return 0, total
	}
	start := (page - 1) * perPage
	if start >= total {
		return total, total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return start, end
}

// JSONResponse attempts to set the status code, c, and marshal the given interface, d, into a response that
// is written to the given ResponseWriter.
func JSONResponse(w http.ResponseWriter, d interface{}, c int) {
	dj, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		http.Error(w, "Error creating JSON response", http.StatusInternalServerError)
		log.Error(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)
	fmt.Fprintf(w, "%s", dj)
}
