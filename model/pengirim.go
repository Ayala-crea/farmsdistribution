package model

type Pengirim struct {
	ID             int    `json:"id"`
	Nama           string `json:"nama"`
	Email          string `json:"email"`
	NoTelp         string `json:"no_telp"`
	Alamat         string `json:"alamat"`
	PlatKendaraan  string `json:"plat_kendaraan"`
	TypeKendaraan  string `json:"type_kendaraan"`
	WarnaKendaraan string `json:"warna_kendaraan"`
	FarmId         int    `json:"farm_id"`
	Password       string `json:"password"`
}
