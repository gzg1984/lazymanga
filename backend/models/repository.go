package models

import "time"

type Repository struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement;index" json:"id"`
	RepoUUID           string    `gorm:"index" json:"repo_uuid"`
	Name               string    `json:"name"`
	Basic              bool      `gorm:"not null;default:false" json:"basic"`
	RootPath           string    `json:"root_path"`
	DBFile             string    `json:"db_filename"`
	IsInternal         bool      `gorm:"not null" json:"is_internal"`
	ExternalDeviceName string    `json:"external_device_name"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
