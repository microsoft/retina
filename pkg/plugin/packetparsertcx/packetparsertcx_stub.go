// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build !linux

package packetparsertcx

// IsTCXSupported reports whether the kernel supports TCX attachment.
// On non-Linux platforms it always returns false (TCX is Linux-only).
func IsTCXSupported() bool {
	return false
}
