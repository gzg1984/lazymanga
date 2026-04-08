package models

import "time"

// RepoTypeDef stores global repository type templates in lazymanga.db.
type RepoTypeDef struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Key                string    `gorm:"not null;uniqueIndex;size:64" json:"key"`
	Name               string    `gorm:"not null;size:120" json:"name"`
	Description        string    `gorm:"not null;default:''" json:"description"`
	Enabled            bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder          int       `gorm:"not null;default:0" json:"sort_order"`
	AddButton          bool      `gorm:"not null;default:false" json:"add_button"`
	AddDirectoryButton bool      `gorm:"not null;default:false" json:"add_directory_button"`
	DeleteButton       bool      `gorm:"not null;default:false" json:"delete_button"`
	AutoNormalize      bool      `gorm:"not null;default:false" json:"auto_normalize"`
	ShowMD5            bool      `gorm:"not null;default:false" json:"show_md5"`
	ShowSize           bool      `gorm:"not null;default:false" json:"show_size"`
	SingleMove         bool      `gorm:"not null;default:false" json:"single_move"`
	ManualEditorMode   string    `gorm:"column:manual_editor_mode;not null;default:legacy-type-editor;size:32" json:"manual_editor_mode"`
	RuleBookName       string    `gorm:"not null;default:noop;size:64" json:"rulebook_name"`
	RuleBookVersion    string    `gorm:"not null;default:v1;size:32" json:"rulebook_version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (RepoTypeDef) TableName() string {
	return "repo_type_defs"
}
