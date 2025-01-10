package model

import (
	"time"

	"gorm.io/gorm"
)

type MenuAccess struct {
	ID             int             `gorm:"primaryKey;autoIncrement" json:"id"`
	MenuID         int             `gorm:"not null" json:"menu_id"`
	ParentID       *int            `gorm:"" json:"parent_id"` // Nullable field
	NamaMenu       string          `gorm:"type:varchar(255);not null" json:"nama_menu"`
	RoutesPage     string          `gorm:"type:varchar(255);not null" json:"routes_page"`
	Icon           *string         `gorm:"type:varchar(255)" json:"icon"` // Nullable field
	Sequence       int             `gorm:"not null" json:"sequence"`
	Status         bool            `gorm:"default:true" json:"status"`
	ParentSequence *int            `gorm:"" json:"parent_sequence"` // Nullable field
	CreatedAt      *gorm.DeletedAt `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      *gorm.DeletedAt `gorm:"column:updated_at" json:"updated_at"`
}

type RoleMenu struct {
	RoleID    int       `gorm:"column:role_id;not null" json:"role_id"`
	MenuID    int       `gorm:"column:menu_id;not null" json:"menu_id"`
	ParentID  *int      `gorm:"column:parent_id" json:"parent_id"`     // Nullable field
	Status    int       `gorm:"column:status;default:1" json:"status"` // Default value 1
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`   // Custom timestamp column
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`   // Custom timestamp column
}
