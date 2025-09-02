// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"

	"github.com/microsoft/retina/pkg/enricher"
)

type Module struct{}

func InitModule(_ context.Context, _ enricher.EnricherInterface) *Module {
	return &Module{}
}

func (m *Module) Reconcile(ctx context.Context) {}

func (m *Module) Clear() {}
