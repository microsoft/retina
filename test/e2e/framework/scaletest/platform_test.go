package scaletest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitCountByNodes(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		linuxNodes   int
		windowsNodes int
		wantLinux    int
		wantWindows  int
		wantErr      bool
	}{
		{
			name:         "800/200 split",
			total:        1000,
			linuxNodes:   800,
			windowsNodes: 200,
			wantLinux:    800,
			wantWindows:  200,
		},
		{
			name:        "Linux only",
			total:       100,
			linuxNodes:  100,
			wantLinux:   100,
			wantWindows: 0,
		},
		{
			name:         "rejects empty Windows allocation",
			total:        1,
			linuxNodes:   1,
			windowsNodes: 1,
			wantErr:      true,
		},
		{
			name:    "rejects zero nodes",
			total:   1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linux, windows, err := SplitCountByNodes(tt.total, tt.linuxNodes, tt.windowsNodes)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantLinux, linux)
			require.Equal(t, tt.wantWindows, windows)
			require.Equal(t, tt.total, linux+windows)
		})
	}
}
