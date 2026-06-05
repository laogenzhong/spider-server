package appleauth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	appconfig "spider-server/common/config"
	"strings"
	"sync"
	"time"
)

const (
	appleIssuer        = "https://appleid.apple.com"
	appleJWKSURL       = "https://appleid.apple.com/auth/keys"
	appleTokenURL      = "https://appleid.apple.com/auth/token"
	appleRevokeURL     = "https://appleid.apple.com/auth/revoke"
	maxClientSecretTTL = 15777000 * time.Second
)

var (
	ErrConfigInvalid        = errors.New("apple sign in config is invalid")
	ErrIdentityTokenEmpty   = errors.New("apple identity token is empty")
	ErrIdentityTokenInvalid = errors.New("apple identity token is invalid")
	ErrNonceInvalid         = errors.New("apple identity token nonce is invalid")
	ErrTokenExchangeFailed  = errors.New("apple token exchange failed")
	ErrTokenRevokeFailed    = errors.New("apple token revoke failed")

	defaultClient = NewClient(appconfig.Default().AppleSignIn)
)

type Client struct {
	cfg        appconfig.AppleSignInConfig
	httpClient *http.Client

	keysMu      sync.Mutex
	keys        map[string]*rsa.PublicKey
	keysFetched time.Time
}

type IdentityClaims struct {
	Subject        string
	Email          string
	EmailVerified  bool
	IsPrivateEmail bool
	Nonce          string
	RealUserStatus int64
	ExpiresAt      int64
	IssuedAt       int64
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ,omitempty"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KeyType string `json:"kty"`
	KeyID   string `json:"kid"`
	Use     string `json:"use"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	E       string `json:"e"`
}

func Configure(cfg appconfig.AppleSignInConfig) {
	defaultClient = NewClient(cfg)
}

func DefaultClient() *Client {
	return defaultClient
}

func NewClient(cfg appconfig.AppleSignInConfig) *Client {
	cfg.ApplyEnv()
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		keys: make(map[string]*rsa.PublicKey),
	}
}

func (c *Client) VerifyIdentityToken(ctx context.Context, token string, nonce string) (*IdentityClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrIdentityTokenEmpty
	}
	if strings.TrimSpace(c.cfg.ClientID) == "" {
		return nil, ErrConfigInvalid
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrIdentityTokenInvalid
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrIdentityTokenInvalid
	}
	header := jwtHeader{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrIdentityTokenInvalid
	}
	if header.Algorithm != "RS256" || header.KeyID == "" {
		return nil, ErrIdentityTokenInvalid
	}

	publicKey, err := c.publicKey(ctx, header.KeyID)
	if err != nil {
		return nil, ErrIdentityTokenInvalid
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrIdentityTokenInvalid
	}
	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, ErrIdentityTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrIdentityTokenInvalid
	}
	rawClaims := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &rawClaims); err != nil {
		return nil, ErrIdentityTokenInvalid
	}

	if getString(rawClaims, "iss") != appleIssuer {
		return nil, ErrIdentityTokenInvalid
	}
	if !audienceMatches(rawClaims["aud"], c.cfg.ClientID) {
		return nil, ErrIdentityTokenInvalid
	}

	now := time.Now().Unix()
	exp := getInt64(rawClaims, "exp")
	if exp <= now {
		return nil, ErrIdentityTokenInvalid
	}

	tokenNonce := getString(rawClaims, "nonce")
	if expectedNonce := strings.TrimSpace(nonce); expectedNonce != "" && !nonceMatches(tokenNonce, expectedNonce) {
		return nil, ErrNonceInvalid
	}

	sub := getString(rawClaims, "sub")
	if sub == "" {
		return nil, ErrIdentityTokenInvalid
	}

	return &IdentityClaims{
		Subject:        sub,
		Email:          getString(rawClaims, "email"),
		EmailVerified:  getBool(rawClaims, "email_verified"),
		IsPrivateEmail: getBool(rawClaims, "is_private_email"),
		Nonce:          tokenNonce,
		RealUserStatus: getInt64(rawClaims, "real_user_status"),
		ExpiresAt:      exp,
		IssuedAt:       getInt64(rawClaims, "iat"),
	}, nil
}

func (c *Client) ExchangeAuthorizationCode(ctx context.Context, code string) (*TokenResponse, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	if strings.TrimSpace(c.cfg.TeamID) == "" ||
		strings.TrimSpace(c.cfg.KeyID) == "" ||
		strings.TrimSpace(c.cfg.ClientID) == "" {
		return nil, ErrConfigInvalid
	}

	clientSecret, err := c.clientSecret()
	if err != nil {
		return nil, ErrConfigInvalid
	}

	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, ErrTokenExchangeFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrTokenExchangeFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrTokenExchangeFailed
	}

	tokenResp := &TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(tokenResp); err != nil {
		return nil, ErrTokenExchangeFailed
	}

	return tokenResp, nil
}

func (c *Client) RevokeToken(ctx context.Context, token string, tokenTypeHint string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if strings.TrimSpace(c.cfg.TeamID) == "" ||
		strings.TrimSpace(c.cfg.KeyID) == "" ||
		strings.TrimSpace(c.cfg.ClientID) == "" {
		return ErrConfigInvalid
	}

	clientSecret, err := c.clientSecret()
	if err != nil {
		return ErrConfigInvalid
	}

	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", clientSecret)
	form.Set("token", token)
	if hint := strings.TrimSpace(tokenTypeHint); hint != "" {
		form.Set("token_type_hint", hint)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appleRevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrTokenRevokeFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrTokenRevokeFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrTokenRevokeFailed
	}

	return nil
}

func (c *Client) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.keysMu.Lock()
	defer c.keysMu.Unlock()

	if key, ok := c.keys[kid]; ok && time.Since(c.keysFetched) < 12*time.Hour {
		return key, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appleJWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch apple jwks status=%d", resp.StatusCode)
	}

	keysResp := jwksResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&keysResp); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(keysResp.Keys))
	for _, jwk := range keysResp.Keys {
		key, err := jwk.publicKey()
		if err != nil {
			continue
		}
		keys[jwk.KeyID] = key
	}
	c.keys = keys
	c.keysFetched = time.Now()

	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("apple public key %s not found", kid)
	}
	return key, nil
}

func (j jwkKey) publicKey() (*rsa.PublicKey, error) {
	if j.KeyType != "RSA" || j.KeyID == "" || j.N == "" || j.E == "" {
		return nil, fmt.Errorf("invalid rsa jwk")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}

	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid rsa exponent")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

func (c *Client) clientSecret() (string, error) {
	privateKey, err := c.privateKey()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	ttl := c.cfg.ClientSecretTTLDuration()
	if ttl > maxClientSecretTTL {
		ttl = maxClientSecretTTL
	}

	header := map[string]string{
		"alg": "ES256",
		"kid": c.cfg.KeyID,
	}
	payload := map[string]any{
		"iss": c.cfg.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
		"aud": appleIssuer,
		"sub": c.cfg.ClientID,
	}

	headerPart, err := encodeJWTPart(header)
	if err != nil {
		return "", err
	}
	payloadPart, err := encodeJWTPart(payload)
	if err != nil {
		return "", err
	}

	signingInput := headerPart + "." + payloadPart
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		return "", err
	}

	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (c *Client) privateKey() (*ecdsa.PrivateKey, error) {
	privateKeyPEM := strings.TrimSpace(c.cfg.PrivateKey)
	privateKeyPEM = strings.ReplaceAll(privateKeyPEM, `\n`, "\n")
	if privateKeyPEM == "" && strings.TrimSpace(c.cfg.PrivateKeyPath) != "" {
		data, err := os.ReadFile(strings.TrimSpace(c.cfg.PrivateKeyPath))
		if err != nil {
			return nil, err
		}
		privateKeyPEM = string(data)
	}
	if privateKeyPEM == "" {
		return nil, ErrConfigInvalid
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, ErrConfigInvalid
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrConfigInvalid
	}
	return ecdsaKey, nil
}

func encodeJWTPart(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func audienceMatches(value any, expected string) bool {
	switch aud := value.(type) {
	case string:
		return aud == expected
	case []any:
		for _, item := range aud {
			if text, ok := item.(string); ok && text == expected {
				return true
			}
		}
	}
	return false
}

func nonceMatches(tokenNonce string, expected string) bool {
	if tokenNonce == expected {
		return true
	}
	sum := sha256.Sum256([]byte(expected))
	return tokenNonce == fmt.Sprintf("%x", sum[:])
}

func getString(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func getBool(values map[string]any, key string) bool {
	value, ok := values[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	case float64:
		return typed != 0
	default:
		return false
	}
}

func getInt64(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		number, _ := typed.Int64()
		return number
	default:
		return 0
	}
}
