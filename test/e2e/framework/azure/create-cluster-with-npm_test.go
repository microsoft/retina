package azure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWindowsOSSKU(t *testing.T) {
	for _, year := range []string{"2022", "2025"} {
		t.Run(year, func(t *testing.T) {
			sku, err := windowsOSSKU(year)
			require.NoError(t, err)
			require.Equal(t, "Windows"+year, string(sku))
		})
	}
	_, err := windowsOSSKU("2026")
	require.ErrorContains(t, err, "unsupported Windows year")
}
