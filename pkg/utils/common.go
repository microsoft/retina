// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const (
	contentTypeStr     = "Content-Type"
	defaultContentType = "application/json; charset=UTF-8"
)

// TODO: Steven to add reverse lookup code here

func DecodeRequestBody(request *http.Request, iface interface{}) (err error) {
	if request.Body != nil {
		err = json.NewDecoder(request.Body).Decode(&iface)
	} else {
		err = fmt.Errorf("nil request.body")
	}

	return err
}

func EncodeResponseBody(w http.ResponseWriter, iface interface{}) error {
	w.Header().Set(contentTypeStr, defaultContentType)
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(iface)
}

// Inspired by https://github.com/mauriciovasquezbernal/talks/blob/1f2080afe731949a033330c0adc290be8f3fc06d/2022-ebpf-training/2022-10-13/drop/main.go .
func Uint32Ptr(v uint32) *uint32 {
	return &v
}

func StringPtr(v string) *string {
	return &v
}

// Exponential backoff retry logic.
func Retry(f func() error, retry int) (err error) {
	for i := 0; i < retry; i++ {
		err = f()
		if err == nil {
			return nil
		}
		t := int64(math.Pow(2, float64(i)))
		time.Sleep(time.Duration(t) * time.Second)
	}
	return err
}

func CompareStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}

	bMap := make(map[string]bool)
	for _, v := range b {
		bMap[v] = true
	}

	if len(aMap) != len(bMap) {
		return false
	}

	for k := range aMap {
		if _, ok := bMap[k]; !ok {
			return false
		}
	}

	return true
}
