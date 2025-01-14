package format

import (
	"strconv"
	"strings"
)

func FormatCurrency(value float64) string {
	// Konversi angka menjadi string dengan dua desimal
	strValue := strconv.FormatFloat(value, 'f', 2, 64)

	// Pisahkan bagian desimal dan ribuan
	parts := strings.Split(strValue, ".")
	intPart := parts[0]     // Bagian sebelum titik
	decimalPart := parts[1] // Bagian setelah titik

	// Tambahkan pemisah ribuan
	var result []string
	for len(intPart) > 3 {
		result = append([]string{intPart[len(intPart)-3:]}, result...)
		intPart = intPart[:len(intPart)-3]
	}
	result = append([]string{intPart}, result...)

	// Gabungkan kembali bagian integer dan desimal
	return "Rp." + strings.Join(result, ",") + "." + decimalPart
}
