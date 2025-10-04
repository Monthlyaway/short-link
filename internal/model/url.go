package model

import (
	"time"
)

// URLMapping represents a URL mapping record
type URLMapping struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	ShortCode   string     `gorm:"uniqueIndex;type:varchar(15);not null" json:"short_code"`
	OriginalURL string     `gorm:"type:varchar(2048);not null" json:"original_url"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	ExpiredAt   *time.Time `gorm:"index" json:"expired_at,omitempty"`
	VisitCount  uint64     `gorm:"default:0" json:"visit_count"`
	Status      int8       `gorm:"default:1" json:"status"` // 1: active, 0: disabled
}

// TableName specifies the table name for URLMapping
func (URLMapping) TableName() string {
	return "url_mappings"
}

// IsExpired checks if the URL mapping is expired
func (u *URLMapping) IsExpired() bool {
	if u.ExpiredAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiredAt)
}

// IsActive checks if the URL mapping is active
func (u *URLMapping) IsActive() bool {
	return u.Status == 1 && !u.IsExpired()
}

// VisitLog represents a visit log record
type VisitLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ShortCode string    `gorm:"index;type:varchar(15);not null" json:"short_code"`
	VisitedAt time.Time `gorm:"autoCreateTime;index" json:"visited_at"`
	IP        string    `gorm:"type:varchar(45)" json:"ip,omitempty"`
	UserAgent string    `gorm:"type:varchar(512)" json:"user_agent,omitempty"`
}

// TableName specifies the table name for VisitLog
func (VisitLog) TableName() string {
	return "visit_logs"
}
