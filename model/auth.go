package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Users struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Nama      string             `bson:"nama,omitempty" json:"nama,omitempty"`
	No_Telp   string             `bson:"no_telp,omitempty" json:"no_telp,omitempty"`
	Email     string             `bson:"email,omitempty" json:"email,omitempty"`
	Alamat    string             `bson:"alamat,omitempty" json:"alamat,omitempty"`
	Role      string             `bson:"role,omitempty" json:"role,omitempty"`
	Password  string             `bson:"password,omitempty" json:"password,omitempty"`
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

type GoogleCredential struct {
	Token        string    `bson:"token"`
	RefreshToken string    `bson:"refresh_token"`
	TokenURI     string    `bson:"token_uri"`
	ClientID     string    `bson:"client_id"`
	ClientSecret string    `bson:"client_secret"`
	Scopes       []string  `bson:"scopes"`
	Expiry       time.Time `bson:"expiry"`
}

type Akun struct {
	ID        int       `gorm:"type:uuid;default:uuid_generate_v4()" json:"id_user"`
	Nama      string    `gorm:"type:varchar(100);not null" json:"nama"`
	NoTelp    string    `gorm:"type:varchar(15)" json:"no_telp"`
	Email     string    `gorm:"type:varchar(100);unique;not null" json:"email"`
	RoleID    int       `gorm:"type:int;not null;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"id_role"` // Foreign key to role table
	Password  string    `gorm:"type:varchar(255);not null" json:"password"`
	CreatedAt time.Time `gorm:"type:timestamp;default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;default:current_timestamp" json:"updated_at"`
}

type Role struct {
	ID       int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Rolename string  `gorm:"type:varchar(255);not null" json:"name_role"`
	Desc     *string `gorm:"type:text" json:"deskripsi"`
	Status   bool    `gorm:"default:true" json:"status"`
}

func (Role) TableName() string {
	return "role"
}
