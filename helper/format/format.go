package format

import (
	"strconv"
	"strings"
)

func FormatCurrency(value float64) string {
	// Konversi angka menjadi string dengan dua desimal
	strValue := strconv.FormatFloat(value, 'f', 0, 64)

	// Tambahkan pemisah ribuan
	var result []string
	for len(strValue) > 3 {
		result = append([]string{strValue[len(strValue)-3:]}, result...)
		strValue = strValue[:len(strValue)-3]
	}
	result = append([]string{strValue}, result...)

	return "Rp." + strings.Join(result, ",")
}
