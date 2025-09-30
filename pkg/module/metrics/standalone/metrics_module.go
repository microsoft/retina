// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"

	"github.com/microsoft/retina/pkg/enricher/base"
)

type Module struct{}

func InitModule(_ context.Context, _ base.EnricherInterface) *Module {
	return &Module{}
}

func (m *Module) Reconcile(_ context.Context) {}

func (m *Module) Clear() {}
