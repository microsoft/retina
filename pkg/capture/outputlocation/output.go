// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import "context"

type Location interface {
	// Name returns the name of the output location.
	Name() string
	// Enabled checks whether a output location is enabled.
	Enabled() bool
	// Output outputs source file to the location specified by the users.
	Output(ctx context.Context, srcFilePath string) error
}
