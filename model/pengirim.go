package model

type Pengirim struct {
	ID             int    `json:"id"`
	Nama           string `json:"nama"`
	Email          string `json:"email"`
	NoTelp         string `json:"no_telp"`
	Alamat         string `json:"alamat"`
	PlatKendaraan  string `json:"vehicle_plate"`
	TypeKendaraan  string `json:"vehicle_type"`
	WarnaKendaraan string `json:"vehicle_color"`
	FarmId         int    `json:"farm_id"`
	Password       string `json:"password"`
}
