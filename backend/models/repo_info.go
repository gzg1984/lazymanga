package models

import "time"

// RepoInfo stores self-describing repository metadata inside repo.db.
// Each repo.db must contain exactly one row identified by ID=1.
type RepoInfo struct {
	ID            uint      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	RepoUUID      string    `gorm:"not null;uniqueIndex" json:"repo_uuid"`
	Name          string    `gorm:"not null" json:"name"`
	Basic         bool      `gorm:"not null;default:false" json:"basic"`
	AddButton     bool      `gorm:"not null;default:false" json:"add_button"`
	DeleteButton  bool      `gorm:"not null;default:false" json:"delete_button"`
	AutoNormalize bool      `gorm:"not null;default:false" json:"auto_normalize"`
	ShowMD5       bool      `gorm:"not null;default:false" json:"show_md5"`
	ShowSize      bool      `gorm:"not null;default:false" json:"show_size"`
	SingleMove    bool      `gorm:"not null;default:false" json:"single_move"`
	SchemaVersion int       `gorm:"not null;default:1" json:"schema_version"`
	FlagsJSON     string    `gorm:"not null;default:{}" json:"flags_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (RepoInfo) TableName() string {
	return "repo_info"
}
