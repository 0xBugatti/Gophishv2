package models

import (
	"errors"
	"time"
)

// ErrURLTemplateNameNotSpecified indicates the name for the URL template was not specified
var ErrURLTemplateNameNotSpecified = errors.New("URL template name not specified")

// ErrURLTemplateURLNotSpecified indicates the URL for the URL template was not specified
var ErrURLTemplateURLNotSpecified = errors.New("URL template URL not specified")

// URLTemplate contains the attributes for a URL template
type URLTemplate struct {
	Id           int64     `json:"id"`
	UserId       int64     `json:"-"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Category     string    `json:"category"`
	IsPreset     bool      `json:"is_preset"`
	ModifiedDate time.Time `json:"modified_date"`
}

// Validate performs validation on a URL template
func (u *URLTemplate) Validate() error {
	if u.Name == "" {
		return ErrURLTemplateNameNotSpecified
	}
	if u.URL == "" {
		return ErrURLTemplateURLNotSpecified
	}
	return nil
}

// GetURLTemplates returns the URL templates owned by the given user.
func GetURLTemplates(uid int64) ([]URLTemplate, error) {
	uts := []URLTemplate{}
	err := db.Where("user_id=?", uid).Find(&uts).Error
	return uts, err
}

// GetURLTemplate returns the URL template, if it exists, specified by the given id and user_id.
func GetURLTemplate(id int64, uid int64) (URLTemplate, error) {
	ut := URLTemplate{}
	err := db.Where("user_id=? and id=?", uid, id).First(&ut).Error
	return ut, err
}

// GetURLTemplateByName returns the URL template, if it exists, specified by the given name and user_id.
func GetURLTemplateByName(n string, uid int64) (URLTemplate, error) {
	ut := URLTemplate{}
	err := db.Where("user_id=? and name=?", uid, n).First(&ut).Error
	return ut, err
}

// PostURLTemplate creates a new URL template in the database.
func PostURLTemplate(u *URLTemplate) error {
	err := u.Validate()
	if err != nil {
		return err
	}
	u.ModifiedDate = time.Now().UTC()
	return db.Save(u).Error
}

// PutURLTemplate edits an existing URL template in the database.
func PutURLTemplate(u *URLTemplate) error {
	u.ModifiedDate = time.Now().UTC()
	return db.Model(&URLTemplate{}).Where("id=? and user_id=?", u.Id, u.UserId).Updates(map[string]interface{}{
		"name":          u.Name,
		"url":           u.URL,
		"category":      u.Category,
		"is_preset":     u.IsPreset,
		"modified_date": u.ModifiedDate,
	}).Error
}

// DeleteURLTemplate deletes an existing URL template in the database.
func DeleteURLTemplate(id int64, uid int64) error {
	return db.Delete(URLTemplate{Id: id, UserId: uid}).Error
}

// DeleteURLTemplates deletes multiple URL templates by their IDs for a given user.
func DeleteURLTemplates(ids []int64, uid int64) error {
	return db.Where("id IN (?) AND user_id = ?", ids, uid).Delete(&URLTemplate{}).Error
}

// TableName specifies the database tablename for Gorm to use
func (u URLTemplate) TableName() string {
	return "url_templates"
}
