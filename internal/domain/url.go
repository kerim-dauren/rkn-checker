package domain

import (
	"net"
	"strings"
)

type URL struct {
	original   string
	normalized string
}

func NewURL(rawURL string) (*URL, error) {
	if rawURL == "" {
		return nil, ErrEmptyURL
	}

	return &URL{
		original: rawURL,
	}, nil
}

func (u *URL) Original() string {
	return u.original
}

func (u *URL) Normalized() string {
	return u.normalized
}

func (u *URL) SetNormalized(normalized string) {
	u.normalized = normalized
}

func (u *URL) IsValid() bool {
	return u.original != "" && u.normalized != ""
}

func IsValidDomain(domain string) bool {
	if domain == "" {
		return false
	}

	if len(domain) > 253 {
		return false
	}

	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if part == "" || len(part) > 63 {
			return false
		}

		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}

	return true
}

func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
