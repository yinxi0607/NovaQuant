package service

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/domain"
)

type AuthService struct {
	enabled          bool
	username         string
	password         string
	passwordSHA256   string
	tokenTTL         time.Duration
	tokenSecret      []byte
	privateKey       *rsa.PrivateKey
	publicKeyPEM     string
	encryptionMethod string
}

type authTokenClaims struct {
	Username string `json:"sub"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

func NewAuthService(cfg config.Config) (*AuthService, error) {
	service := &AuthService{
		enabled:        cfg.AuthEnabled,
		username:       strings.TrimSpace(cfg.AuthUsername),
		password:       cfg.AuthPassword,
		passwordSHA256: strings.ToLower(strings.TrimSpace(cfg.AuthPasswordSHA256)),
		tokenTTL:       cfg.AuthTokenTTL,
	}
	if !service.enabled {
		return service, nil
	}
	if service.username == "" {
		return nil, errors.New("AUTH_USERNAME is required when auth is enabled")
	}
	if service.password == "" && service.passwordSHA256 == "" {
		return nil, errors.New("AUTH_PASSWORD or AUTH_PASSWORD_SHA256 is required when auth is enabled")
	}
	secret := strings.TrimSpace(cfg.AuthTokenSecret)
	if secret == "" {
		secret = randomSecret(32)
	}
	service.tokenSecret = []byte(secret)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate login rsa key: %w", err)
	}
	service.privateKey = privateKey
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal login public key: %w", err)
	}
	service.publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	service.encryptionMethod = "RSA-OAEP-256"
	return service, nil
}

func (s *AuthService) Enabled() bool {
	return s != nil && s.enabled
}

func (s *AuthService) Config() domain.AuthConfigResponse {
	if !s.Enabled() {
		return domain.AuthConfigResponse{Enabled: false}
	}
	return domain.AuthConfigResponse{
		Enabled:   true,
		Username:  s.username,
		Algorithm: s.encryptionMethod,
		PublicKey: s.publicKeyPEM,
	}
}

func (s *AuthService) Login(_ context.Context, req domain.AuthLoginRequest) (domain.AuthLoginResponse, error) {
	if !s.Enabled() {
		return domain.AuthLoginResponse{}, errors.New("auth is disabled")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Username)), []byte(s.username)) != 1 {
		return domain.AuthLoginResponse{}, errors.New("invalid username or password")
	}
	password, err := s.resolvePassword(req)
	if err != nil {
		return domain.AuthLoginResponse{}, err
	}
	if !s.verifyPassword(password) {
		return domain.AuthLoginResponse{}, errors.New("invalid username or password")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	token, err := s.signToken(authTokenClaims{
		Username: s.username,
		IssuedAt: now.Unix(),
		Expires:  expiresAt.Unix(),
	})
	if err != nil {
		return domain.AuthLoginResponse{}, err
	}
	return domain.AuthLoginResponse{
		Token: token,
		Session: domain.AuthSession{
			Username:  s.username,
			ExpiresAt: expiresAt,
		},
		ExpiresAt: expiresAt,
	}, nil
}

func (s *AuthService) ValidateToken(token string) (domain.AuthSession, error) {
	if !s.Enabled() {
		return domain.AuthSession{}, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.AuthSession{}, errors.New("missing token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return domain.AuthSession{}, errors.New("invalid token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return domain.AuthSession{}, errors.New("invalid token payload")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return domain.AuthSession{}, errors.New("invalid token signature")
	}
	expected := signBytes(parts[0], s.tokenSecret)
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return domain.AuthSession{}, errors.New("invalid token signature")
	}
	var claims authTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return domain.AuthSession{}, errors.New("invalid token claims")
	}
	if claims.Username != s.username {
		return domain.AuthSession{}, errors.New("token subject mismatch")
	}
	if time.Now().UTC().Unix() >= claims.Expires {
		return domain.AuthSession{}, errors.New("token expired")
	}
	return domain.AuthSession{
		Username:  claims.Username,
		ExpiresAt: time.Unix(claims.Expires, 0).UTC(),
	}, nil
}

func (s *AuthService) resolvePassword(req domain.AuthLoginRequest) (string, error) {
	if strings.TrimSpace(req.EncryptedPassword) != "" {
		raw, err := base64.StdEncoding.DecodeString(req.EncryptedPassword)
		if err != nil {
			return "", errors.New("invalid encrypted password encoding")
		}
		plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, s.privateKey, raw, nil)
		if err != nil {
			return "", errors.New("invalid encrypted password")
		}
		return string(plain), nil
	}
	if strings.TrimSpace(req.Password) != "" {
		return req.Password, nil
	}
	return "", errors.New("password is required")
}

func (s *AuthService) verifyPassword(password string) bool {
	if s.password != "" && subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) == 1 {
		return true
	}
	if s.passwordSHA256 != "" {
		sum := sha256.Sum256([]byte(password))
		return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(s.passwordSHA256)) == 1
	}
	return false
}

func (s *AuthService) signToken(claims authTokenClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := signBytes(encodedPayload, s.tokenSecret)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func signBytes(value string, secret []byte) []byte {
	mac := hmac.New(crypto.SHA256.New, secret)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func randomSecret(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
