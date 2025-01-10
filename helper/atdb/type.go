package atdb

type DBInfo struct {
	DBString string
	DBName   string
}

type NewLiburNasional struct {
	Tanggal    string `json:"tanggal"`
	Keterangan string `json:"keterangan"`
	IsCuti     bool   `json:"is_cuti"`
}
