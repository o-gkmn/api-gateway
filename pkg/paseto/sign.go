package paseto

import (
	"crypto/ed25519"
	"encoding/base64"
)

const headerStr = "v4.public."

func SignV4Public(privateKey ed25519.PrivateKey, payload, footer, implicit []byte) string {
	byteH := []byte(headerStr)

	m2 := PAE(byteH, payload, footer, implicit)

	sig := ed25519.Sign(privateKey, m2)
	body := make([]byte, 0, len(payload)+len(sig))
	body = append(body, payload...)
	body = append(body, sig...)

	base64Body := base64.RawURLEncoding.EncodeToString(body)
	base64Footer := base64.RawURLEncoding.EncodeToString(footer)

	if len(base64Footer) > 0 {
		base64Footer = "." + base64Footer
	}

	token := headerStr + base64Body + base64Footer

	return token
}
