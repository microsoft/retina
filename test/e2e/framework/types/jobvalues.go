package types

import (
	"fmt"
	"sync"
)

var (
	ErrValueAlreadySet = fmt.Errorf("parameter already set in values")
	ErrEmptyValue      = fmt.Errorf("empty parameter not found in values")
)

type JobValues struct {
	RWLock sync.RWMutex
	kv     map[string]string
}

func (j *JobValues) New() *JobValues {
	return &JobValues{
		kv: make(map[string]string),
	}
}

func (j *JobValues) Contains(key string) bool {
	j.RWLock.RLock()
	defer j.RWLock.RUnlock()
	_, ok := j.kv[key]
	return ok
}

func (j *JobValues) Get(key string) string {
	j.RWLock.RLock()
	defer j.RWLock.RUnlock()
	return j.kv[key]
}

func (j *JobValues) SetGet(key, value string) (string, error) {
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
