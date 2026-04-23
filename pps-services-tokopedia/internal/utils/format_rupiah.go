package utils

import (
	"fmt"
	"strings"
)

func FormatRupiah(amount float64) string {
	// Konversi ke int dulu (kalau memang tidak pakai desimal)
	val := int64(amount)

	// Format dengan pemisah ribuan
	formatted := fmt.Sprintf("%d", val)

	// Tambahkan titik setiap 3 digit dari belakang
	n := len(formatted)
	if n > 3 {
		var sb strings.Builder
		mod := n % 3
		if mod > 0 {
			sb.WriteString(formatted[:mod])
			sb.WriteString(".")
		}
		for i := mod; i < n; i += 3 {
			sb.WriteString(formatted[i : i+3])
			if i+3 < n {
				sb.WriteString(".")
			}
		}
		formatted = sb.String()
	}

	return "Rp" + formatted
}
