package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

type Store struct {
	db *sql.DB
}

// NewStore recebe a conexao pronta para facilitar testes e troca de banco.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate garante que as tabelas necessarias existam.
func (s *Store) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			revoked_at DATETIME,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`

	_, err := s.db.Exec(query)
	return err
}

// CreateUser salva um usuario novo com senha ja protegida por hash.
func (s *Store) CreateUser(name string, email string, password string) (User, error) {
	passwordHash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}

	result, err := s.db.Exec(
		"INSERT INTO users (name, email, password_hash, created_at) VALUES (?, ?, ?, ?)",
		name,
		email,
		passwordHash,
		time.Now().UTC(),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, ErrEmailAlreadyExists
		}
		return User{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}

	return User{ID: id, Name: name, Email: email, PasswordHash: passwordHash}, nil
}

// FindUserByEmail busca o usuario usado no login.
func (s *Store) FindUserByEmail(email string) (User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, name, email, password_hash FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}

// FindUserByID carrega o usuario autenticado pela rota protegida.
func (s *Store) FindUserByID(id int64) (User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, name, email FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Name, &user.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidToken
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}

// SaveRefreshToken armazena apenas o hash do refresh token.
func (s *Store) SaveRefreshToken(userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?)",
		userID,
		tokenHash(token),
		expiresAt,
		time.Now().UTC(),
	)
	return err
}

// UseRefreshToken valida e revoga o token antigo para fazer rotacao.
func (s *Store) UseRefreshToken(token string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	var userID int64
	var expiresAt time.Time
	err = tx.QueryRow(
		"SELECT id, user_id, expires_at FROM refresh_tokens WHERE token_hash = ? AND revoked_at IS NULL",
		tokenHash(token),
	).Scan(&id, &userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidToken
	}
	if err != nil {
		return 0, err
	}
	if time.Now().UTC().After(expiresAt) {
		return 0, ErrInvalidToken
	}

	if _, err := tx.Exec("UPDATE refresh_tokens SET revoked_at = ? WHERE id = ?", time.Now().UTC(), id); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return userID, nil
}

// RevokeRefreshToken invalida o refresh token no logout.
func (s *Store) RevokeRefreshToken(token string) error {
	_, err := s.db.Exec(
		"UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL",
		time.Now().UTC(),
		tokenHash(token),
	)
	return err
}

type TokenManager struct {
	secret []byte
}

type Claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Expires int64  `json:"exp"`
}

// NewTokenManager cria o assinador de access tokens JWT.
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

// GenerateAccessToken cria um JWT HS256 com expiracao curta.
func (m *TokenManager) GenerateAccessToken(user User) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := Claims{
		Subject: strconv.FormatInt(user.ID, 10),
		Email:   user.Email,
		Expires: time.Now().UTC().Add(15 * time.Minute).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + m.sign(unsigned), nil
}

// ParseAccessToken valida assinatura, expiracao e extrai as claims.
func (m *TokenManager) ParseAccessToken(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(m.sign(unsigned)), []byte(parts[2])) {
		return Claims{}, ErrInvalidToken
	}

	rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if time.Now().UTC().Unix() > claims.Expires {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}

// sign assina o conteudo do JWT com HMAC-SHA256.
func (m *TokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type Handler struct {
	store        *Store
	tokenManager *TokenManager
}

// NewHandler injeta store e token manager nos handlers HTTP.
func NewHandler(store *Store, tokenManager *TokenManager) *Handler {
	return &Handler{store: store, tokenManager: tokenManager}
}

// RegisterRoutes centraliza as rotas de autenticacao.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("GET /me", h.AuthMiddleware(http.HandlerFunc(h.Me)).ServeHTTP)
}

// Register cria um novo usuario e ja devolve tokens.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	name := strings.TrimSpace(request.Name)
	email := strings.ToLower(strings.TrimSpace(request.Email))
	if name == "" || !validEmail(email) || len(request.Password) < 6 {
		writeError(w, http.StatusBadRequest, "invalid user data")
		return
	}

	user, err := h.store.CreateUser(name, email, request.Password)
	if errors.Is(err, ErrEmailAlreadyExists) {
		writeError(w, http.StatusConflict, "email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.writeTokenResponse(w, http.StatusCreated, user)
}

// Login valida credenciais e gera novo access token + refresh token.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	user, err := h.store.FindUserByEmail(strings.ToLower(strings.TrimSpace(request.Email)))
	if err != nil || !checkPassword(request.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.writeTokenResponse(w, http.StatusOK, user)
}

// Refresh troca um refresh token valido por um novo par de tokens.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	userID, err := h.store.UseRefreshToken(request.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := h.store.FindUserByID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	h.writeTokenResponse(w, http.StatusOK, user)
}

// Logout revoga o refresh token informado.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.store.RevokeRefreshToken(request.RefreshToken); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me retorna os dados do usuario autenticado.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey{}).(int64)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.store.FindUserByID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// AuthMiddleware exige Authorization: Bearer <token>.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := h.tokenManager.ParseAccessToken(rawToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := withUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeTokenResponse emite access token e refresh token juntos.
func (h *Handler) writeTokenResponse(w http.ResponseWriter, statusCode int, user User) {
	accessToken, err := h.tokenManager.GenerateAccessToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	refreshToken, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if err := h.store.SaveRefreshToken(user.ID, refreshToken, time.Now().UTC().Add(7*24*time.Hour)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	writeJSON(w, statusCode, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

func validEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func hashPassword(password string) (string, error) {
	salt, err := randomBytes(16)
	if err != nil {
		return "", err
	}

	hash := passwordDigest(salt, []byte(password))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

func checkPassword(password string, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	return hmac.Equal(expected, passwordDigest(salt, []byte(password)))
}

// passwordDigest aplica varias rodadas de SHA-256 com salt.
func passwordDigest(salt []byte, password []byte) []byte {
	data := append(append([]byte{}, salt...), password...)
	sum := sha256.Sum256(data)
	for range 100_000 {
		sum = sha256.Sum256(sum[:])
	}
	return sum[:]
}

func randomToken() (string, error) {
	bytes, err := randomBytes(32)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func randomBytes(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}

	return bytes, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func withUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

type userIDKey struct{}
