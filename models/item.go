package models

import (
	log "github.com/gophish/gophish/logger"
	"github.com/sirupsen/logrus"
)

type Item struct {
	Id         int64  `gorm:"primaryKey"`
	ItemType   string `gorm:"primaryKey"`
	ItemTypeID int64  `gorm:"primaryKey"`
	Teams      []Team `gorm:"many2many:item_teams;"`
}

// ItemTeams is used for a many-to-many relationship between 1..* Item and 1..* Teams
type ItemTeams struct {
	TeamId int64 `json:"-"`
	ItemId int64 `json:"-"`
}

// GetItem gives back the Information about the Item. Teams included
func GetItem(Iid int64, i string, uid int64) (Item, error) {
	item := Item{}
	err := db.Preload("Teams").Find(&item, "item_type_id = ? and item_type = ?", Iid, i).Error
	if err != nil {
		log.Error(err)
		return item, err
	}

	return item, nil
}

// RelateItemAndTeam relates the item with the given teams
func RelateItemAndTeam(i string, id int64, teams []Team, uid int64) error {
	// Fetch item's existing teams from database.
	ts, err := GetItemTeams(id, i, uid)
	if err != nil {
		log.WithFields(logrus.Fields{
			"item_id": id,
		}).Error("Error getting teams from item")
		return err
	}

	// Preload the caches
	cacheNew := make(map[string]int64, len(teams))
	for _, t := range teams {
		cacheNew[t.Name] = t.Id
	}

	cacheExisting := make(map[string]int64, len(ts))
	for _, t := range ts {
		cacheExisting[t.Name] = t.Id
	}
	tx := db.Begin()
	// Check existing team, removing any that are no longer related to the item.
	for _, t := range ts {
		if _, ok := cacheNew[t.Name]; ok {
			continue
		}
		// If the team does not relate to the item any longer, we remove it
		err := tx.Where("item_id=?", id).Delete(&ItemTeams{}).Error
		if err != nil {
			tx.Rollback()
			log.WithFields(logrus.Fields{
				"name": t.Name,
			}).Error("Error deleting team")
		}
	}
	// Add any teams that are not in the database yet.
	for _, nt := range teams {
		// If the team already exists in the database, we skip it.
		if _, ok := cacheExisting[nt.Name]; ok {
			continue
		}
		// Add team if not in database
		err = RelateTeamToItem(tx, i, id, nt.Id)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// GetItemTeams shows the Teams assigned to the Item
func GetItemTeams(Iid int64, i string, uid int64) ([]TeamSummary, error) {
	tids := []int64{}

	err := db.Table("item_teams").Where("item_id = ?", Iid).Pluck("team_id", &tids).Error
	if err != nil {
		log.Error(err)
		return nil, err
	}
	ts := make([]TeamSummary, len(tids))
	for i, tid := range tids {
		ts[i], err = GetTeam(tid)

		if err != nil {
			log.Error(err)
		}
	}
	return ts, nil
}
