package models

type ISOs struct {
	ID        uint   `gorm:"primaryKey;autoIncrement;index" json:"id"`
	UUID      string `json:"uuid"`
	FileName  string `json:"filename"`
	Path      string `json:"path"`
	MountPath string `json:"mountpath"`
	MD5       string `json:"md5"`
	Tags      string `json:"tags"`
	IsMounted bool   `json:"ismounted"`
}
