package ciliumutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestErrNotFound(t *testing.T) {
	err := ErrNotFound{}
	require.True(t, apierrors.IsNotFound(err))
}
