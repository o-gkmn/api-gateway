package main

import (
	"api-gateway/pkg/keys"
	"api-gateway/pkg/paseto"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	claims := map[string]any{
		"sub":   "user_1",
		"iss":   getenv("JWT_ISSUER", "https://auth.local.test"),
		"aud":   getenv("JWT_AUDIENCE", "api-gateway"),
		"roles": []string{"admin"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	}

	jsonBytes, err := json.Marshal(claims)
	if err != nil {
		log.Fatalf("JSON'a dönüştürülürken hata oluştu: %v", err)
	}

	m := string(jsonBytes)
	f := ""
	i := ""

	privateKey, err := keys.LoadEd25519Private("keys/paseto_private.pem")
	if err != nil {
		log.Fatal(err)
	}

	mBytes := []byte(m)
	fBytes := []byte(f)
	iBytes := []byte(i)

	token := paseto.SignV4Public(privateKey, mBytes, fBytes, iBytes)
	fmt.Println(token)
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
