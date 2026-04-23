package models

const UnknownRepoISOSizeBytes int64 = -1

// RepoISO uses a dedicated table for repository mode indexing.
// Keep schema close to ISOs for now; repo mode may evolve independently.
type RepoISO struct {
	ID          uint   `gorm:"primaryKey;autoIncrement;index" json:"id"`
	UUID        string `json:"uuid"`
	FileName    string `json:"filename"`
	Path        string `gorm:"index" json:"path"`
	MountPath   string `json:"mountpath"`
	IsMissing   bool   `gorm:"column:is_missing;not null;default:false;index" json:"is_missing"`
	IsDirectory bool   `gorm:"column:is_directory;not null;default:false;index" json:"is_directory"`
	IsOS        bool   `gorm:"column:is_os;not null;default:false" json:"is_os"`
	// Keep the historical spelling for compatibility with existing API callers.
	IsEntertament bool           `gorm:"column:is_entertament;not null;default:false" json:"is_entertament"`
	MD5           string         `json:"md5"`
	SizeBytes     int64          `gorm:"column:size_bytes;default:-1" json:"size_bytes"`
	MetadataJSON  string         `gorm:"column:metadata_json;type:text;default:''" json:"metadata_json,omitempty"`
	ItemKind      string         `gorm:"-" json:"item_kind,omitempty"`
	Metadata      map[string]any `gorm:"-" json:"metadata,omitempty"`
	Tags          string         `json:"tags"`
	IsMounted     bool           `json:"ismounted"`
}

func (RepoISO) TableName() string {
	return "repoisos"
}
