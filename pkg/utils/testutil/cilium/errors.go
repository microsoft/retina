//go:unit

package ciliumutil

import (
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrAlreadyExists  = errors.New("already exists")
	ErrNotImplemented = errors.New("not implemented")
)

const ErrCodeNotFound = 404

type ErrNotFound struct{}

func (e ErrNotFound) Error() string {
	return "not found on API server"
}

func (e ErrNotFound) Status() v1.Status {
	return v1.Status{
		Reason: v1.StatusReasonNotFound,
		Code:   ErrCodeNotFound,
	}
}
