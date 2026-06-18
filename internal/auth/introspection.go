package auth

import (
	"api-gateway/internal/reqctx"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type IntrospectionResponse struct {
	Active   bool   `json:"active"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	Scope    string `json:"scope"`
	Exp      int64  `json:"exp"`
	IAT      int64  `json:"iat"`
	NBF      int64  `json:"nbf"`
	Sub      string `json:"sub"`
	Aud      string `json:"aud"`
	Iss      string `json:"iss"`
	JTI      string `json:"jti"`
}

type IntrospectionVerifier struct {
	endpoint     string
	clientID     string
	clientSecret string
	client       *http.Client
	issuer       string
	audience     string
	now          func() time.Time
}

func NewIntrospectionVerifier(endpoint, clientId, clientSecret, issuer, audience string,
	now func() time.Time) *IntrospectionVerifier {
	iv := &IntrospectionVerifier{
		endpoint:     endpoint,
		clientID:     clientId,
		clientSecret: clientSecret,
		issuer:       issuer,
		audience:     audience,
		now:          now,
	}

	if now == nil {
		iv.now = time.Now
	}

	iv.client = &http.Client{Timeout: 10 * time.Second}

	return iv
}

func (v *IntrospectionVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {

	formData := url.Values{}
	formData.Set("token", token)

	req, err := http.NewRequestWithContext(ctx, "POST", v.endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(v.clientID, v.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection: status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	r := IntrospectionResponse{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, err
	}

	if !r.Active {
		return nil, errors.New("token is inactive")
	}

	if v.issuer != "" && r.Iss != "" && r.Iss != v.issuer {
		return nil, errors.New("issuer mismatch")
	}
	if v.audience != "" && r.Aud != "" && r.Aud != v.audience {
		return nil, errors.New("audience mismatch")
	}

	if r.Exp > 0 && v.now().After(time.Unix(r.Exp, 0)) {
		return nil, errors.New("token expired")
	}

	roles := strings.Fields(r.Scope)
	claims := &reqctx.Claims{
		Sub:   r.Sub,
		Roles: roles,
		Exp:   r.Exp,
	}

	return claims, nil
}
