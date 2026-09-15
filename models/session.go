package models

import (
	"time"

	"github.com/gophish/gophish/auth"
)

// Session represents a server-side session record.
// A session token is written to the DB on login and deleted on logout.
// Middleware validates that the token stored in the cookie exists in the DB,
// so a stolen cookie is rejected the moment the legitimate user logs out.
type Session struct {
	Id        int64     `json:"-" gorm:"column:id; primary_key:yes"`
	UserId    int64     `json:"-" gorm:"column:user_id"`
	Token     string    `json:"-" gorm:"column:token"`
	CreatedAt time.Time `json:"-" gorm:"column:created_at"`
}

// TableName sets the database table name for GORM.
func (s Session) TableName() string {
	return "sessions"
}

// maxSessionsPerUser caps concurrent server-side sessions per user to prevent
// a DoS via unbounded session table growth (#9351).
const maxSessionsPerUser = 10

// CreateSession generates a new server-side session token for the given user,
// persists it to the database, and returns the token string.
// If the user already has maxSessionsPerUser sessions, the oldest is evicted first.
func CreateSession(userId int64) (string, error) {
	var count int
	db.Model(&Session{}).Where("user_id = ?", userId).Count(&count)
	if count >= maxSessionsPerUser {
		var oldest Session
		if err := db.Where("user_id = ?", userId).Order("created_at asc").First(&oldest).Error; err == nil {
			db.Delete(&oldest)
		}
	}
	token := auth.GenerateSecureKey(auth.APIKeyLength)
	s := Session{
		UserId:    userId,
		Token:     token,
		CreatedAt: time.Now().UTC(),
	}
	err := db.Save(&s).Error
	return token, err
}

// GetSessionByToken returns the Session record for the given token, or an error
// if the token does not exist (i.e. the session has been invalidated).
func GetSessionByToken(token string) (Session, error) {
	s := Session{}
	err := db.Where("token = ?", token).First(&s).Error
	return s, err
}

// DeleteSession removes the session record identified by token from the DB.
// This is called on logout to invalidate the session immediately.
func DeleteSession(token string) error {
	return db.Where("token = ?", token).Delete(&Session{}).Error
}

// DeleteSessionsForUser removes all session records for the given user ID.
// Useful for "log out everywhere" and on account deletion.
func DeleteSessionsForUser(userId int64) error {
	return db.Where("user_id = ?", userId).Delete(&Session{}).Error
}
