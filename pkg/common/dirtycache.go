// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import "sync"

type DirtyCache struct {
	// dirty is a map of dirty objects.
	toAdd map[string]interface{}
	// toDelete is a map of dirty objects.
	toDelete map[string]interface{}

	sync.RWMutex
}

func NewDirtyCache() *DirtyCache {
	return &DirtyCache{
		toAdd:    make(map[string]interface{}),
		toDelete: make(map[string]interface{}),
		RWMutex:  sync.RWMutex{},
	}
}

func (d *DirtyCache) ToAdd(key string, obj interface{}) {
	d.Lock()
	defer d.Unlock()
	delete(d.toDelete, key)
	d.toAdd[key] = obj
}

func (d *DirtyCache) ToDelete(key string, obj interface{}) {
	d.Lock()
	defer d.Unlock()
	delete(d.toAdd, key)
	d.toDelete[key] = obj
}

func (d *DirtyCache) GetAddList() []interface{} {
	d.RLock()
	defer d.RUnlock()

	al := make([]interface{}, 0, len(d.toAdd))
	for _, v := range d.toAdd {
		al = append(al, v)
	}
	return al
}

func (d *DirtyCache) GetDeleteList() []interface{} {
	d.RLock()
	defer d.RUnlock()

	dl := make([]interface{}, 0, len(d.toDelete))
	for _, v := range d.toDelete {
		dl = append(dl, v)
	}
	return dl
}

func (d *DirtyCache) ClearAdd() {
	d.Lock()
	defer d.Unlock()
	d.toAdd = make(map[string]interface{})
}

func (d *DirtyCache) ClearDelete() {
	d.Lock()
	defer d.Unlock()
	d.toDelete = make(map[string]interface{})
}
