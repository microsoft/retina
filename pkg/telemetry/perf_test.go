package telemetry

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerformanceCounter(t *testing.T) {
	p, err := NewPerfProfile()
	require.NoError(t, err)

	for i := 0; i < 1000000; i++ {
		fmt.Printf("i: %d, rand: %d\n", i, rand.IntN(100))
	}

	props, err := p.GetCPUUsage()
	require.NoError(t, err)
	require.NotEmpty(t, props[userCPUSeconds], "user time should not be zero")
	require.NotEmpty(t, props[sysCPUSeconds], "system time should not be zero")
}
