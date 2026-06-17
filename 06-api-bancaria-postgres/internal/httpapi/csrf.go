package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const csrfHeader = "X-CSRF-Token"

type CSRFManager struct {
	secret []byte
	maxAge time.Duration
}

// NewCSRFManager cria o gerador e validador de tokens CSRF.
func NewCSRFManager(secret string) *CSRFManager {
	return &CSRFManager{
		secret: []byte(secret),
		maxAge: 2 * time.Hour,
	}
}

// Generate cria um token assinado com timestamp e nonce aleatorio.
func (m *CSRFManager) Generate() (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}

	timestamp := time.Now().Unix()
	nonce := hex.EncodeToString(nonceBytes)
	payload := fmt.Sprintf("%d:%s", timestamp, nonce)
	signature := m.sign(payload)

	return payload + ":" + signature, nil
}

// Valid confere formato, validade de tempo e assinatura HMAC.
func (m *CSRFManager) Valid(token string) bool {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return false
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}

	createdAt := time.Unix(timestamp, 0)
	if time.Since(createdAt) > m.maxAge || time.Until(createdAt) > time.Minute {
		return false
	}

	payload := parts[0] + ":" + parts[1]
	expectedSignature := m.sign(payload)
	return hmac.Equal([]byte(expectedSignature), []byte(parts[2]))
}

// Middleware bloqueia operacoes de escrita sem token CSRF valido.
func (m *CSRFManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.Valid(r.Header.Get(csrfHeader)) {
			writeError(w, http.StatusForbidden, "invalid csrf token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sign assina o payload usando HMAC-SHA256.
func (m *CSRFManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
