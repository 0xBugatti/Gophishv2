package models

import (
	"errors"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Role represents a user role within Gophish. Each user has a single role
// which maps to a set of permissions.

type Team struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Items       []Item `json:"-" gorm:"many2many:item_teams;"`
	Users       []User `json:"-" gorm:"many2many:team_users;"`
	Role        Role   `json:"role" gorm:"association_autoupdate:false;association_autocreate:false"`
	RoleID      int64  `json:"-" gorm:"-"`
}

type TeamSummary struct {
	Id          int64         `json:"id" gorm:"primaryKey"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Users       []UserSummary `json:"users"`
}

// TeamUsers is used for a many-to-many relationship between 1..* Team and 1..* Users
type TeamUsers struct {
	UserId int64 `json:"-" gorm:"primaryKey"`
	TeamId int64 `json:"team_id" gorm:"primaryKey"`
	Role   Role  `json:"role" gorm:"association_autoupdate:false;association_autocreate:false"`
	RoleId int64 `json:"-"`
}

// ErrPageNameNotSpecified is thrown if the name of the landing page is blank.
var ErrTeamNameNotSpecified = errors.New("Team Name not specified")
var ErrNoUsersSpecified = errors.New("No users specified")

// Validate ensures that a Team contains the appropriate details
func (t *TeamSummary) Validate() error {
	switch {
	case t.Name == "":
		return ErrTeamNameNotSpecified
	case len(t.Users) == 0:
		return ErrNoUsersSpecified
	}
	return nil
}

// Checks if the user_id is in any of the teams
func checkForValue(userValue int64, teams []Team) bool {

	//traverse through the map
	for _, team := range teams {
		for _, user := range team.Users {
			//check if present value is equals to userValue
			if user.Id == userValue {
				return true
			}
		}
	}
	return false
}

// convertTeamSummary converts a team into the teamsummary format. This is used to we dont expose information from users
func convertTeamSummary(t Team) TeamSummary {

	ts := TeamSummary{Id: t.Id, Name: t.Name, Description: t.Description}
	for _, user := range t.Users {
		for _, team := range user.Teams {
			if team.Id == t.Id {
				u := UserSummary{Id: user.Id, Username: user.Username, Role: team.Role}
				ts.Users = append(ts.Users, u)
			}
		}
	}
	return ts
}

// GetTeams gives back all the teams
func GetTeams() ([]TeamSummary, error) {
	teams := []Team{}
	err := db.Preload("Users.Teams.Role").Find(&teams).Error
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ts := []TeamSummary{}
	for _, team := range teams {
		ts = append(ts, convertTeamSummary(team))
	}
	return ts, err
}

// GetTeam returns the team, if it exists, specified by the given id.
func GetTeam(id int64) (TeamSummary, error) {
	t := Team{}
	ts := TeamSummary{}
	err := db.Preload("Users.Teams.Role").First(&t, "id =?", id).Error
	if err != nil {
		log.Error(err)
		return ts, err
	}
	ts = convertTeamSummary(t)
	return ts, nil
}

// GetTeamUsers returns the users related to the team
func GetTeamUsers(id int64) ([]User, error) {
	t := Team{}
	err := db.Preload("Users").First(&t, "id =?", id).Error
	if err != nil {
		log.Error(err)
	}

	return t.Users, err
}

// InsertTeamUsers relates the given user to the team
func InsertTeamUsers(tx *gorm.DB, u UserSummary, tid int64) error {
	err := tx.Save(&TeamUsers{UserId: u.Id, RoleId: u.Role.ID, TeamId: tid}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	return nil
}

// PostTeam creates a new team in the database.
func PostTeam(t *TeamSummary) error {
	if err := t.Validate(); err != nil {
		return err
	}
	// Insert the team into the DB
	nt := Team{Name: t.Name, Description: t.Description}
	tx := db.Begin()
	err := tx.Save(&nt).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}
	for _, u := range t.Users {
		err = InsertTeamUsers(tx, u, nt.Id)
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	t.Id = nt.Id
	return nil
}

// PutTeam updates the given team if found in the database.
func PutTeam(t *TeamSummary) error {
	// Fetch team's existing users from database.
	us, err := GetTeamUsers(t.Id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"team_id": t.Id,
		}).Error("Error getting users from team")
		return err
	}
	// Preload the caches
	cacheNew := make(map[string]int64, len(t.Users))
	for _, u := range t.Users {
		cacheNew[u.Username] = u.Id
	}

	cacheExisting := make(map[string]int64, len(us))
	for _, u := range us {
		cacheExisting[u.Username] = u.Id
	}
	tx := db.Begin()
	// Check existing users, removing any that are no longer in the team.
	for _, u := range us {
		if _, ok := cacheNew[u.Username]; ok {
			continue
		}

		// If the user does not exist in the team any longer, we remove it
		err := tx.Where("team_id=? and user_id=?", t.Id, u.Id).Delete(&TeamUsers{}).Error
		if err != nil {
			tx.Rollback()
			log.WithFields(logrus.Fields{
				"name": u.Username,
			}).Error("Error deleting email")
		}
	}
	// Add any users that are not in the database yet.
	for _, nu := range t.Users {
		// If the user is already related to the team, we should just update
		// the record with the latest information.
		if id, ok := cacheExisting[nu.Username]; ok {
			nu.Id = id
			err = UpdateTeamUser(tx, nu, t.Id)
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
			continue
		}
		// Otherwise, add user
		err = InsertTeamUsers(tx, nu, t.Id)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
	}
	err = tx.Save(&Team{Id: t.Id, Name: t.Name, Description: t.Description}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// UpdateTeamUser updates the given user information.
func UpdateTeamUser(tx *gorm.DB, user UserSummary, tid int64) error {
	userInfo := map[string]interface{}{
		"role_id": user.Role.ID,
	}
	err := tx.Model(&TeamUsers{}).Where("team_id = ? AND user_id = ?", tid, user.Id).Updates(userInfo).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"role": user.Role.Slug,
		}).Error("Error updating user role")
	}
	return err
}

// RelateTeamToItem relates a team to the given item
func RelateTeamToItem(tx *gorm.DB, i string, id int64, tid int64) error {
	err := tx.Save(&ItemTeams{ItemId: id, TeamId: tid}).Error

	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteTeam deletes a given team by team ID and user ID
func DeleteTeam(t *TeamSummary) error {
	tx := db.Begin()
	// Delete all the team_users entries for this team
	err := tx.Where("team_id=?", t.Id).Delete(&TeamUsers{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	// Delete all the item_team entries for this team
	err = tx.Where("team_id=?", t.Id).Delete(&ItemTeams{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	// Delete the team itself
	err = tx.Delete(Team{Id: t.Id}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
