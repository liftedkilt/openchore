package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ResolveAuth merges the auth section of the config file with environment
// variables and validates it. It never returns nil.
//
// Environment variables:
//
//	OPENCHORE_PUBLIC_URL                      overrides auth.public_url
//	OIDC_ISSUER, OIDC_CLIENT_ID,              add (or replace) one provider
//	OIDC_CLIENT_SECRET, OIDC_PROVIDER_ID,     without editing the file
//	OIDC_PROVIDER_NAME, OIDC_SCOPES, OIDC_PROMPT
func ResolveAuth(cfg *Config) (*AuthConfig, error) {
	out := &AuthConfig{}
	if cfg != nil && cfg.Auth != nil {
		c := *cfg.Auth
		c.OIDC = append([]OIDCProviderConfig(nil), cfg.Auth.OIDC...)
		out = &c
	}
	out.PublicURL = os.ExpandEnv(out.PublicURL)
	if v := os.Getenv("OPENCHORE_PUBLIC_URL"); v != "" {
		out.PublicURL = v
	}
	out.PublicURL = strings.TrimRight(out.PublicURL, "/")

	for i := range out.OIDC {
		p := &out.OIDC[i]
		p.Issuer = os.ExpandEnv(p.Issuer)
		p.ClientID = os.ExpandEnv(p.ClientID)
		p.ClientSecret = os.ExpandEnv(p.ClientSecret)
	}

	if issuer := os.Getenv("OIDC_ISSUER"); issuer != "" {
		p := OIDCProviderConfig{
			ID:           envOr("OIDC_PROVIDER_ID", "oidc"),
			Name:         envOr("OIDC_PROVIDER_NAME", "Single sign-on"),
			Issuer:       issuer,
			ClientID:     os.Getenv("OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			Prompt:       os.Getenv("OIDC_PROMPT"),
		}
		if s := os.Getenv("OIDC_SCOPES"); s != "" {
			p.Scopes = ParseScopes(s)
		}
		replaced := false
		for i := range out.OIDC {
			if out.OIDC[i].ID == p.ID {
				out.OIDC[i] = p
				replaced = true
			}
		}
		if !replaced {
			out.OIDC = append(out.OIDC, p)
		}
	}

	seen := map[string]bool{}
	for i := range out.OIDC {
		p := &out.OIDC[i]
		if !ValidProviderID(p.ID) {
			return nil, fmt.Errorf("auth.oidc[%d]: id %q must be lowercase letters, digits, '-' or '_'", i, p.ID)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("auth.oidc: duplicate provider id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Issuer == "" || p.ClientID == "" {
			return nil, fmt.Errorf("auth.oidc %q: issuer and client_id are required", p.ID)
		}
		if p.Name == "" {
			p.Name = p.ID
		}
		p.Scopes = NormalizeScopes(p.Scopes)
	}

	for _, d := range []struct{ name, v string }{
		{"kiosk_session_ttl", out.KioskSessionTTL},
		{"personal_session_ttl", out.PersonalSessionTTL},
	} {
		if d.v == "" {
			continue
		}
		if dur, err := time.ParseDuration(d.v); err != nil || dur <= 0 {
			return nil, fmt.Errorf("auth.%s: invalid duration %q", d.name, d.v)
		}
	}
	return out, nil
}

// ValidProviderID reports whether id can name an OIDC provider: it appears
// in the callback URL and is stored with linked identities.
func ValidProviderID(id string) bool { return providerIDPattern.MatchString(id) }

// ParseScopes splits a space- or comma-separated scope list.
func ParseScopes(s string) []string {
	return strings.Fields(strings.ReplaceAll(s, ",", " "))
}

// NormalizeScopes applies the default scopes and makes sure "openid" is
// requested.
func NormalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email"}
	}
	if !contains(scopes, "openid") {
		return append([]string{"openid"}, scopes...)
	}
	return scopes
}

// SessionTTLs returns the parsed session lifetimes (zero means default).
func (a *AuthConfig) SessionTTLs() (kiosk, personal time.Duration) {
	kiosk, _ = time.ParseDuration(a.KioskSessionTTL)
	personal, _ = time.ParseDuration(a.PersonalSessionTTL)
	return kiosk, personal
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
