package model

type Profile struct {
	ID        int    `json:"id_user"`
	Nama      string `json:"nama"`
	NoTelp    string `json:"no_telp"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	RoleID    int    `json:"id_role"`
	RoleName  string `json:"role_name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	AddressID *int   `json:"address_id,omitempty"`
	Address   struct {
		Street     *string `json:"street,omitempty"`
		City       *string `json:"city,omitempty"`
		State      *string `json:"state,omitempty"`
		PostalCode *string `json:"postal_code,omitempty"`
		Country    *string `json:"country,omitempty"`
	} `json:"address"`
	Location []float64 `json:"location,omitempty"`
	Image    *string   `json:"image,omitempty"`
}
