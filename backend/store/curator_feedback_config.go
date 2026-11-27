package store

import (
	"database/sql"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const curatorFeedbackConfigSchema = `CREATE TABLE IF NOT EXISTS curator_feedback_config (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	enabled INTEGER NOT NULL,
	address TEXT NOT NULL,
	token TEXT NOT NULL
)`

var (
	ErrInvalidCuratorFeedbackAddress = errors.New("invalid Curator HTTPS origin")
	ErrInvalidCuratorFeedbackToken   = errors.New("invalid Curator token")
	ErrMissingCuratorFeedbackConfig  = errors.New("Curator integration needs an address and token")
)

type CuratorFeedbackConfig struct {
	Enabled bool
	Address string
	Token   string `json:"-"`
}

// CuratorFeedbackOrigin returns the canonical HTTPS origin of a source URL.
func CuratorFeedbackOrigin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || strings.Contains(raw, "#") {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || strings.HasSuffix(u.Host, ":") || strings.Contains(host, "%") ||
		(strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return ""
	}
	port := u.Port()
	if port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return ""
		}
		if number != 443 {
			return "https://" + net.JoinHostPort(host, strconv.FormatUint(number, 10))
		}
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "https://" + host
}

func (s *Store) GetCuratorFeedbackConfig() (CuratorFeedbackConfig, error) {
	var cfg CuratorFeedbackConfig
	err := s.db.QueryRow("SELECT enabled, address, token FROM curator_feedback_config WHERE id = 1").
		Scan(&cfg.Enabled, &cfg.Address, &cfg.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	return cfg, err
}

// A nil token preserves the old token only at the same origin. An empty string clears it.
func (s *Store) UpdateCuratorFeedbackConfig(enabled bool, address string, token *string) (CuratorFeedbackConfig, error) {
	if strings.TrimSpace(address) == "" {
		empty := ""
		enabled, token = false, &empty
	}
	address, token, err := normalizeCuratorFeedbackInput(address, token)
	if err != nil {
		return CuratorFeedbackConfig{}, err
	}
	var cfg CuratorFeedbackConfig
	err = s.db.QueryRow(`
		WITH next(token) AS (
			SELECT COALESCE(?1, (SELECT token FROM curator_feedback_config WHERE id = 1 AND address = ?2), '')
		)
		INSERT INTO curator_feedback_config (id, enabled, address, token)
		SELECT 1, ?3, ?2, token FROM next
		WHERE NOT ?3 OR (?2 <> '' AND token <> '')
		ON CONFLICT (id) DO UPDATE SET enabled = excluded.enabled, address = excluded.address, token = excluded.token
		RETURNING enabled, address, token`, token, address, enabled).Scan(&cfg.Enabled, &cfg.Address, &cfg.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, ErrMissingCuratorFeedbackConfig
	}
	return cfg, err
}

func normalizeCuratorFeedbackInput(address string, token *string) (string, *string, error) {
	address = strings.TrimSpace(address)
	if address != "" {
		origin := CuratorFeedbackOrigin(address)
		if origin == "" || len(address) > 2048 {
			return "", nil, ErrInvalidCuratorFeedbackAddress
		}
		u, _ := url.Parse(address)
		if (u.EscapedPath() != "" && u.EscapedPath() != "/") || u.RawQuery != "" || u.ForceQuery {
			return "", nil, ErrInvalidCuratorFeedbackAddress
		}
		address = origin
	}
	if token != nil {
		value := strings.TrimSpace(*token)
		if len(value) > 4096 || strings.IndexFunc(value, func(c rune) bool { return c < 32 || c == 127 }) >= 0 {
			return "", nil, ErrInvalidCuratorFeedbackToken
		}
		if value != "" && address == "" {
			return "", nil, ErrMissingCuratorFeedbackConfig
		}
		token = &value
	}
	return address, token, nil
}

// CuratorFeedbackConnection resolves draft credentials without saving anything.
func (s *Store) CuratorFeedbackConnection(address string, token *string) (CuratorFeedbackConfig, error) {
	address, token, err := normalizeCuratorFeedbackInput(address, token)
	if err != nil {
		return CuratorFeedbackConfig{}, err
	}
	cfg := CuratorFeedbackConfig{Address: address}
	if token != nil {
		cfg.Token = *token
	} else {
		saved, err := s.GetCuratorFeedbackConfig()
		if err != nil {
			return CuratorFeedbackConfig{}, err
		}
		if saved.Address == address {
			cfg.Token = saved.Token
		}
	}
	if cfg.Address == "" || cfg.Token == "" {
		return CuratorFeedbackConfig{}, ErrMissingCuratorFeedbackConfig
	}
	return cfg, nil
}
