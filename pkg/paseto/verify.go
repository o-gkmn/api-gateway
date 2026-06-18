package paseto

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
)

func VerifyV4Public(m, f string, key ed25519.PublicKey) ([]byte, error) {
	hBytes := []byte(headerStr)
	var iBytes []byte

	decodedPayload, err := base64.RawURLEncoding.DecodeString(m)
	if err != nil {
		return nil, err
	}

	if len(decodedPayload) < ed25519.SignatureSize {
		return nil, errors.New("token too short")
	}

	mBytes := decodedPayload[:len(decodedPayload)-ed25519.SignatureSize]
	signature := decodedPayload[len(decodedPayload)-ed25519.SignatureSize:]
	fBytes, err := base64.RawURLEncoding.DecodeString(f)

	if err != nil {
		return nil, err
	}

	paeInput := PAE(hBytes, mBytes, fBytes, iBytes)

	ok := ed25519.Verify(key, paeInput, signature)
	if !ok {
		return nil, errors.New("invalid token")
	}

	return mBytes, nil
}
