package model

type Peternakan struct {
	User_id            int64   `json:"user_id"`
	Nama_peternakan    string  `json:"nama_peternakan"`
	Street             string  `json:"street"`
	City               string  `json:"city"`
	State              string  `json:"state"`
	PostalCode         string  `json:"postal_code"`
	Country            string  `json:"country"`
	Lat                float64 `json:"lat"`
	Lon                float64 `json:"lon"`
	PeternakanImageURL string  `json:"image_farm"`
}

type Farms struct {
	ID              int64   `json:"id"`
	User_id         int64   `json:"user_id"`
	Nama_peternakan string  `json:"nama_peternakan"`
	Name            string  `json:"name"`
	Farm_Type       string  `json:"farm_type"`
	Street          string  `json:"street"`
	City            string  `json:"city"`
	State           string  `json:"state"`
	PostalCode      string  `json:"postal_code"`
	Country         string  `json:"country"`
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	FamrsImageURL   string  `json:"image_farm"`
	ProfileImageURL string  `json:"image_profile"`
	PhonenumberFam  string  `json:"phonenumber_farm"`
	PhonenumberUser string  `json:"phonenumber_user"`
	Email           string  `json:"email"`
	Website         string  `json:"website"`
	Description     string  `json:"description"`
}
