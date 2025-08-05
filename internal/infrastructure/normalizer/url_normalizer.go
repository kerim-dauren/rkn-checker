package normalizer

import (
	"net"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/idna"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

var (
	portRegex = regexp.MustCompile(`:\d+$`)
	wwwRegex  = regexp.MustCompile(`^www\.`)
)

type URLNormalizer struct {
	idnProfile *idna.Profile
}

func NewURLNormalizer() *URLNormalizer {
	return &URLNormalizer{
		idnProfile: idna.New(
			idna.ValidateLabels(true),
			idna.VerifyDNSLength(true),
			idna.StrictDomainName(false),
		),
	}
}

func (n *URLNormalizer) Normalize(rawURL string) (string, error) {
	if rawURL == "" {
		return "", domain.ErrEmptyURL
	}

	rawURL = strings.TrimSpace(rawURL)

	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", domain.ErrInvalidURL
	}

	if parsedURL.Host == "" {
		return "", domain.ErrInvalidURL
	}

	host := parsedURL.Host

	host = n.removePort(host)

	host = strings.ToLower(host)

	if ip := net.ParseIP(host); ip != nil {
		return n.normalizeIP(ip), nil
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		ipv6 := strings.Trim(host, "[]")
		if ip := net.ParseIP(ipv6); ip != nil {
			return n.normalizeIP(ip), nil
		}
		return "", domain.ErrInvalidIP
	}

	normalized, err := n.normalizeDomain(host)
	if err != nil {
		return "", err
	}

	return normalized, nil
}

func (n *URLNormalizer) removePort(host string) string {
	if strings.Contains(host, ":") && !strings.Contains(host, "::") {
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			portPart := host[idx:]
			if portRegex.MatchString(portPart) {
				return host[:idx]
			}
		}
	}
	return host
}

func (n *URLNormalizer) normalizeDomain(host string) (string, error) {
	ascii, err := n.idnProfile.ToASCII(host)
	if err != nil {
		return "", domain.ErrNormalizationFailed
	}

	normalized := strings.ToLower(ascii)

	if wwwRegex.MatchString(normalized) {
		withoutWWW := strings.TrimPrefix(normalized, "www.")
		if domain.IsValidDomain(withoutWWW) {
			normalized = withoutWWW
		}
	}

	if !domain.IsValidDomain(normalized) {
		return "", domain.ErrInvalidDomain
	}

	return normalized, nil
}

func (n *URLNormalizer) normalizeIP(ip net.IP) string {
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

func (n *URLNormalizer) NormalizeURL(domainURL *domain.URL) error {
	if domainURL == nil {
		return domain.ErrInvalidURL
	}

	normalized, err := n.Normalize(domainURL.Original())
	if err != nil {
		return err
	}

	domainURL.SetNormalized(normalized)
	return nil
}
