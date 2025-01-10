package metric

import (
	"crypto/rand"
	"log"
)

func generateSymmetricKey() []byte {
	key := make([]byte, 32) // 32 byte panjang
	_, err := rand.Read(key)
	if err != nil {
		log.Fatal("Failed to generate random key:", err)
	}
	return key
}

var symmetricKey = generateSymmetricKey() // Hasilkan kunci simetris dengan panjang 32 byte
