package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultPassword          = "station-demo"
	defaultSessionTTL        = 12 * time.Hour
	passwordHashIterations   = 60_000
	passwordHashLength       = 32
	passwordSaltLength       = 16
	sessionTokenLength       = 32
	sessionTimestampFormat   = time.RFC3339
	passwordDerivationDomain = "silent-outposts-station-password-v1"
)

var (
	ErrInvalidCredentials = errors.New("invalid station credentials")
	ErrUnauthorized       = errors.New("station authentication required")
)

type contextKey string

const accountContextKey contextKey = "station-account"

type Config struct {
	DefaultPassword string
	SessionTTL      time.Duration
}

type Service struct {
	db              *sql.DB
	defaultPassword string
	sessionTTL      time.Duration
}

type Account struct {
	StationID string `json:"stationId"`
	Username  string `json:"username"`
}

type StationAccount struct {
	StationID  string `json:"stationId"`
	Username   string `json:"username"`
	SenderType string `json:"senderType"`
	Location   string `json:"location"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Account   Account `json:"account"`
	Token     string  `json:"token"`
	ExpiresAt string  `json:"expiresAt"`
}

type SessionResponse struct {
	Account   Account `json:"account"`
	ExpiresAt string  `json:"expiresAt"`
}

func New(db *sql.DB, config Config) *Service {
	password := strings.TrimSpace(config.DefaultPassword)
	if password == "" {
		password = defaultPassword
	}
	ttl := config.SessionTTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &Service{db: db, defaultPassword: password, sessionTTL: ttl}
}

func ContextWithAccount(ctx context.Context, account Account) context.Context {
	return context.WithValue(ctx, accountContextKey, account)
}

func AccountFromContext(ctx context.Context) (Account, bool) {
	account, ok := ctx.Value(accountContextKey).(Account)
	return account, ok
}

// EnsureStationAccounts creates one login account for every known station. It
// never rewrites an existing account, so local password changes survive imports.
func (s *Service) EnsureStationAccounts(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sender_id FROM senders
        WHERE sender_id NOT IN (SELECT station_id FROM station_accounts)
        ORDER BY sender_id`)
	if err != nil {
		return 0, fmt.Errorf("query stations without accounts: %w", err)
	}
	defer rows.Close()

	stationIDs := make([]string, 0)
	for rows.Next() {
		var stationID string
		if err := rows.Scan(&stationID); err != nil {
			return 0, fmt.Errorf("scan station account candidate: %w", err)
		}
		stationIDs = append(stationIDs, stationID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate station account candidates: %w", err)
	}
	if len(stationIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin station account seed: %w", err)
	}
	defer tx.Rollback()
	for _, stationID := range stationIDs {
		salt, encodedHash, err := newPasswordHash(s.defaultPassword)
		if err != nil {
			return 0, fmt.Errorf("create password hash for station %q: %w", stationID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO station_accounts
            (station_id, username, password_salt, password_hash)
            VALUES (?, ?, ?, ?)`, stationID, stationID, salt, encodedHash); err != nil {
			return 0, fmt.Errorf("insert station account %q: %w", stationID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit station account seed: %w", err)
	}
	return len(stationIDs), nil
}

func (s *Service) StationAccounts(ctx context.Context) ([]StationAccount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.station_id, a.username, s.sender_type,
        COALESCE(latest.location, '')
        FROM station_accounts a
        JOIN senders s ON s.sender_id = a.station_id
        LEFT JOIN broadcasts latest ON latest.broadcast_id = (
            SELECT latest_b.broadcast_id FROM broadcasts latest_b
            WHERE latest_b.sender_id = a.station_id
            ORDER BY latest_b.timestamp DESC, latest_b.broadcast_id DESC LIMIT 1
        )
        ORDER BY a.station_id`)
	if err != nil {
		return nil, fmt.Errorf("query station accounts: %w", err)
	}
	defer rows.Close()
	accounts := make([]StationAccount, 0)
	for rows.Next() {
		var account StationAccount
		if err := rows.Scan(&account.StationID, &account.Username, &account.SenderType, &account.Location); err != nil {
			return nil, fmt.Errorf("scan station account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate station accounts: %w", err)
	}
	return accounts, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResponse, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	var account Account
	var encodedSalt, encodedHash string
	err := s.db.QueryRowContext(ctx, `SELECT station_id, username, password_salt, password_hash
        FROM station_accounts WHERE username = ?`, username).Scan(
		&account.StationID, &account.Username, &encodedSalt, &encodedHash,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("query station account: %w", err)
	}
	if !verifyPassword(password, encodedSalt, encodedHash) {
		return LoginResponse{}, ErrInvalidCredentials
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return LoginResponse{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.sessionTTL)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM station_sessions WHERE expires_at <= ?`, formatTime(now)); err != nil {
		return LoginResponse{}, fmt.Errorf("delete expired sessions: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO station_sessions
        (token_hash, station_id, created_at, expires_at, last_seen_at)
        VALUES (?, ?, ?, ?, ?)`,
		tokenHash, account.StationID, formatTime(now), formatTime(expiresAt), formatTime(now),
	); err != nil {
		return LoginResponse{}, fmt.Errorf("create station session: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE station_accounts SET last_login_at = ? WHERE station_id = ?`,
		formatTime(now), account.StationID); err != nil {
		return LoginResponse{}, fmt.Errorf("record station login: %w", err)
	}

	return LoginResponse{Account: account, Token: token, ExpiresAt: formatTime(expiresAt)}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (SessionResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return SessionResponse{}, ErrUnauthorized
	}
	tokenHash := hashToken(token)
	now := time.Now().UTC()
	var response SessionResponse
	err := s.db.QueryRowContext(ctx, `SELECT a.station_id, a.username, ss.expires_at
        FROM station_sessions ss
        JOIN station_accounts a ON a.station_id = ss.station_id
        WHERE ss.token_hash = ? AND ss.expires_at > ?`,
		tokenHash, formatTime(now),
	).Scan(&response.Account.StationID, &response.Account.Username, &response.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionResponse{}, ErrUnauthorized
	}
	if err != nil {
		return SessionResponse{}, fmt.Errorf("query station session: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE station_sessions SET last_seen_at = ? WHERE token_hash = ?`,
		formatTime(now), tokenHash); err != nil {
		return SessionResponse{}, fmt.Errorf("touch station session: %w", err)
	}
	return response, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM station_sessions WHERE token_hash = ?`, hashToken(token)); err != nil {
		return fmt.Errorf("delete station session: %w", err)
	}
	return nil
}

func newPasswordHash(password string) (string, string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := derivePasswordHash(password, salt)
	return encodeBytes(salt), encodeBytes(hash), nil
}

func verifyPassword(password, encodedSalt, encodedHash string) bool {
	salt, err := decodeBytes(encodedSalt)
	if err != nil {
		return false
	}
	expected, err := decodeBytes(encodedHash)
	if err != nil {
		return false
	}
	actual := derivePasswordHash(password, salt)
	return hmac.Equal(actual, expected)
}

func derivePasswordHash(password string, salt []byte) []byte {
	return pbkdf2SHA256([]byte(password), append([]byte(passwordDerivationDomain+"\x00"), salt...), passwordHashIterations, passwordHashLength)
}

func pbkdf2SHA256(password, salt []byte, iterations, length int) []byte {
	out := make([]byte, 0, length)
	for block := uint32(1); len(out) < length; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		var number [4]byte
		binary.BigEndian.PutUint32(number[:], block)
		mac.Write(number[:])
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		remaining := length - len(out)
		if remaining > len(t) {
			remaining = len(t)
		}
		out = append(out, t[:remaining]...)
	}
	return out
}

func newSessionToken() (string, string, error) {
	bytes := make([]byte, sessionTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token := encodeBytes(bytes)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return encodeBytes(sum[:])
}

func encodeBytes(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func decodeBytes(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(sessionTimestampFormat)
}
