package config

import (
	"net/http"
)

// Daftar origins yang diizinkan
var Origins = []string{
	"https://kb.pd.my.id",
	"https://go.biz.id",
	"https://jual.in.my.id",
	"http://127.0.0.1:5500",
	"http://127.0.0.1:5173",
	"http://localhost:5173",
}

// Fungsi untuk memeriksa apakah origin diizinkan
func isAllowedOrigin(origin string) bool {
	for _, o := range Origins {
		if o == origin {
			return true
		}
	}
	return false
}

// Middleware CORS
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Periksa apakah origin diizinkan
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Login")
			w.Header().Set("Access-Control-Max-Age", "3600")

			// Tangani preflight request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		} else {
			// Jika origin tidak diizinkan
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Lanjutkan ke handler berikutnya
		next.ServeHTTP(w, r)
	})
}
