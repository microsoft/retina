package types

import (
	"errors"
	"sync"
)

var ErrEmptyRuntimeObject = errors.New("empty value for runtime object key")

type RuntimeObjects struct {
	RWLock sync.RWMutex
	kv     map[string]interface{}
}

func (j *RuntimeObjects) Contains(key string) bool {
	j.RWLock.RLock()
	defer j.RWLock.RUnlock()
	_, ok := j.kv[key]
	return ok
}

func (j *RuntimeObjects) Get(key string) (interface{}, bool) {
	j.RWLock.RLock()
	defer j.RWLock.RUnlock()
	val, ok := j.kv[key]
	return val, ok
}

func (j *RuntimeObjects) SetGet(key string, value interface{}) (interface{}, error) {
	j.RWLock.Lock()
	defer j.RWLock.Unlock()

	_, ok := j.kv[key]

	switch {
	case !ok && value != "":
		j.kv[key] = value
		return value, nil
	case ok && value == "":
		return j.kv[key], nil
	case ok && value != "":
		return "", ErrValueAlreadySet
	}

	return "", ErrEmptyValue
}
