package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	keyPEM, err := os.ReadFile("keys/jwt_private.pem")
	if err != nil {
		log.Fatalf("read private key (önce ./cmd/genkey çalıştırdın mı?): %v", err)
	}
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		log.Fatal(err)
	}

	claims := jwt.MapClaims{
		"sub":   "user_1",
		"iss":   getenv("JWT_ISSUER", "https://auth.local.test"),
		"aud":   getenv("JWT_AUDIENCE", "api-gateway"),
		"roles": []string{"admin"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "dev"

	s, err := token.SignedString(priv)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(s)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
