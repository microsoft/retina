// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"testing"

	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/source/statefile/azure"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		enrichmentMode string
		location       string
		wantType       interface{}
		wantErr        error
	}{
		{
			name:           "valid statefile enrichment type",
			enrichmentMode: "azure-vnet-statefile",
			location:       "azure-vnet.json",
			wantType:       &azure.Statefile{},
			wantErr:        nil,
		},
		{
			name:           "invalid statefile type",
			enrichmentMode: "gcp-vnet-statefile",
			location:       "gcp-vnet.json",
			wantType:       nil,
			wantErr:        ErrUnsupportedStatefileType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := New(tt.enrichmentMode, tt.location)

			if tt.wantErr != nil {
				require.ErrorContains(t, err, tt.wantErr.Error())
				require.Nil(t, src, "expected nil source on error")
			} else {
				require.NoError(t, err, "expected no error")
				require.IsType(t, tt.wantType, src, "statefile source type mismatch")
			}
		})
	}
}
