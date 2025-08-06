package common

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func MapDomainErrorToHTTP(err error) int {
	switch err {
	case domain.ErrEmptyURL:
		return http.StatusBadRequest
	case domain.ErrInvalidURL:
		return http.StatusBadRequest
	case domain.ErrRegistryEntryInvalid:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func MapDomainErrorToGRPC(err error) codes.Code {
	switch err {
	case domain.ErrEmptyURL:
		return codes.InvalidArgument
	case domain.ErrInvalidURL:
		return codes.InvalidArgument
	case domain.ErrRegistryEntryInvalid:
		return codes.InvalidArgument
	default:
		return codes.Internal
	}
}

func NewGRPCError(err error, message string) error {
	code := MapDomainErrorToGRPC(err)
	return status.Error(code, message)
}
