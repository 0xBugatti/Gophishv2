package models

import (
	"time"

	log "github.com/gophish/gophish/logger"
)

// AuditLog represents a single audit trail entry (6.12).
type AuditLog struct {
	Id         int64     `json:"id" gorm:"primary_key"`
	UserId     int64     `json:"user_id"`
	Action     string    `json:"action"` // create, update, delete, launch, login, logout
	ObjectType string    `json:"object_type"`
	ObjectId   int64     `json:"object_id"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details" gorm:"type:text"`
}

// TableName tells GORM to use the singular form to match the migration DDL.
func (AuditLog) TableName() string { return "audit_log" }

// LogAction inserts an audit log entry.
func LogAction(uid int64, action, objectType string, objectId int64, details string) {
	entry := AuditLog{
		UserId:     uid,
		Action:     action,
		ObjectType: objectType,
		ObjectId:   objectId,
		Timestamp:  time.Now().UTC(),
		Details:    details,
	}
	if err := db.Save(&entry).Error; err != nil {
		log.Errorf("audit log write failed: %v", err)
	}
}

// GetAuditLogs returns paginated audit log entries (6.12).
// page and perPage are 1-based; if page==0, returns all.
func GetAuditLogs(page, perPage int) ([]AuditLog, error) {
	logs := []AuditLog{}
	q := db.Order("timestamp desc")
	if page > 0 && perPage > 0 {
		offset := (page - 1) * perPage
		q = q.Offset(offset).Limit(perPage)
	}
	err := q.Find(&logs).Error
	return logs, err
}
