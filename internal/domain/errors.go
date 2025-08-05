package domain

import "errors"

var (
	ErrInvalidURL           = errors.New("invalid URL format")
	ErrEmptyURL             = errors.New("URL cannot be empty")
	ErrUnsupportedProtocol  = errors.New("unsupported protocol")
	ErrInvalidDomain        = errors.New("invalid domain format")
	ErrInvalidIP            = errors.New("invalid IP address format")
	ErrNormalizationFailed  = errors.New("URL normalization failed")
	ErrBlockingRuleInvalid  = errors.New("blocking rule is invalid")
	ErrRegistryEntryInvalid = errors.New("registry entry is invalid")
)
