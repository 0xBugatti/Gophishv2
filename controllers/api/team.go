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

type teamRequest struct {
	Id          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Users       []teamUserRequest `json:"users"`
}

type teamUserRequest struct {
	Id       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Teams handles requests for the /api/teams/ endpoint
func (as *Server) Teams(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetTeams()
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new team and return it as JSON
	case r.Method == "POST":
		tr := &teamRequest{}
		err := json.NewDecoder(r.Body).Decode(&tr)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		//Check if team exists
		_, err = models.GetTeam(tr.Id)
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Team already exists"}, http.StatusConflict)
			log.Error(err)
			return
		}
		t := models.TeamSummary{Id: tr.Id, Name: tr.Name, Description: tr.Description}
		for _, user := range tr.Users {
			role, err := models.GetRoleBySlug(user.Role)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
				return
			}
			t.Users = append(t.Users, models.UserSummary{Id: user.Id, Username: user.Username, Role: role})
		}
		err = models.PostTeam(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, t, http.StatusCreated)
	}
}

func (as *Server) Team(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	ts, err := models.GetTeam(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Team not found"}, http.StatusNotFound)
		return
	}
	switch {
	// GET: Return the team as JSON
	case r.Method == "GET":
		JSONResponse(w, ts, http.StatusOK)
	//DELETE: Delete the team and return success
	case r.Method == "DELETE":
		err = models.DeleteTeam(&ts)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting team"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Team deleted successfully!"}, http.StatusOK)
	//PUT: Update the team and return it as JSON
	case r.Method == "PUT":
		tr := &teamRequest{}
		err = json.NewDecoder(r.Body).Decode(&tr)
		if err != nil {
			log.Errorf("error decoding group: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if tr.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and team_id mismatch"}, http.StatusBadRequest)
			return
		}
		// Use TeamSummary struct to handle the put team.
		t := models.TeamSummary{Id: tr.Id, Name: tr.Name, Description: tr.Description}
		// Check if slug exists
		for _, user := range tr.Users {
			role, err := models.GetRoleBySlug(user.Role)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
				return
			}
			// Add the user to the team summary
			t.Users = append(t.Users, models.UserSummary{Id: user.Id, Username: user.Username, Role: role})
		}
		// update the team
		err = models.PutTeam(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, t, http.StatusOK)
	}
}

// ItemTeams is used to get or post the Teams related to a item.
func (as *Server) ItemTeams(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	item, err := models.GetItem(id, vars["item"], ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
	}
	switch {
	case r.Method == "GET":
		ts, err := models.GetItemTeams(id, vars["item"], ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ts, http.StatusOK)
	//POST: Update the teams related to the item.
	case r.Method == "POST":
		t := []models.Team{}
		// Put the request into a slice of teams.
		err := json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		// Relate Item based on item type.
		err = models.RelateItemAndTeam(vars["item"], item.Id, t, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, t, http.StatusCreated)
	}
}
