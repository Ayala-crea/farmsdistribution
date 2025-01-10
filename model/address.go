package model

type Address struct {
	ID         int    `gorm:"type:int;primary_key;auto_increment" json:"id"`
	Street     string `gorm:"type:varchar(255)" json:"street"`
	City       string `gorm:"type:varchar(100)" json:"city"`
	State      string `gorm:"type:varchar(100)" json:"state"`
	PostalCode string `gorm:"type:varchar(20)" json:"postal_code"`
	Country    string `gorm:"type:varchar(100)" json:"country"`
}
