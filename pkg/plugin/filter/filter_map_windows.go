// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filter

import "net"

type FilterMap struct{}

func Init() (*FilterMap, error) {
	return &FilterMap{}, nil
}

func (f *FilterMap) Add(ips []net.IP) error {
	return nil
}

func (f *FilterMap) Delete(ips []net.IP) error {
	return nil
}

func (f *FilterMap) Close() {
}
