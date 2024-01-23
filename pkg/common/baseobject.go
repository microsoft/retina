// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"fmt"
	"sync"
)

func GetBaseObject(name, namespace string, ips *IPAddresses) BaseObject {
	return BaseObject{
		name:      name,
		namespace: namespace,
		ips:       ips,
		RWMutex:   &sync.RWMutex{},
	}
}

func (b *BaseObject) Key() string {
	return fmt.Sprintf("%s/%s", b.namespace, b.name)
}

func (b *BaseObject) DeepCopy() BaseObject {
	b.RLock()
	defer b.RUnlock()

	newB := BaseObject{
		name:      b.name,
		namespace: b.namespace,
		ips:       b.ips.DeepCopy(),
		RWMutex:   &sync.RWMutex{},
	}

	return newB
}

func (b *BaseObject) Name() string {
	b.RLock()
	defer b.RUnlock()

	return b.name
}

func (b *BaseObject) Namespace() string {
	b.RLock()
	defer b.RUnlock()

	return b.namespace
}

func (b *BaseObject) NamespacedName() string {
	b.RLock()
	defer b.RUnlock()

	return fmt.Sprintf("%s/%s", b.namespace, b.name)
}

func (b *BaseObject) IPs() *IPAddresses {
	b.RLock()
	defer b.RUnlock()

	return b.ips
}
