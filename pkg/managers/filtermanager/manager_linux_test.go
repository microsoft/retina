// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"errors"
	"net"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/filter/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_AddIPs_EmptyCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4(), net.ParseIP("2.2.2.2").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl)
	for _, ip := range ips {
		mockCache.EXPECT().hasKey(ip).Return(false).Times(1)
		mockCache.EXPECT().addIP(ip, r, m).Times(1)
	}

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Add(ips).Return(nil).Times(1)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.AddIPs(ips, r, m)
	require.NoError(t, err)
	mockCache.EXPECT().hasKey(ips[0]).Return(true).Times(1)
	mockCache.EXPECT().hasKey(ips[1]).Return(true).Times(1)
	assert.True(t, f.HasIP(ips[0]))
	assert.True(t, f.HasIP(ips[1]))
}

func Test_AddIPs_CacheHit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4(), net.ParseIP("2.2.2.2").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl)
	for _, ip := range ips {
		mockCache.EXPECT().hasKey(ip).Return(true).Times(1)
		mockCache.EXPECT().addIP(ip, r, m).Times(1)
	}
	f := &FilterManager{
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.AddIPs(ips, r, m)
	assert.NoError(t, err)
}

func Test_AddIPs_MixedCacheHit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4(), net.ParseIP("2.2.2.2").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl)
	// First IP is a cache hit.
	mockCache.EXPECT().hasKey(ips[0]).Return(true).Times(1)
	mockCache.EXPECT().addIP(ips[0], r, m).Times(1)
	// Second IP is a cache miss.
	mockCache.EXPECT().hasKey(ips[1]).Return(false).Times(1)
	mockCache.EXPECT().addIP(ips[1], r, m).Times(1)

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Add([]net.IP{ips[1]}).Return(nil).Times(1)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.AddIPs(ips, r, m)
	assert.NoError(t, err)
}

func Test_AddIPs_Retry(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}
	retry := 3
	expectedErr := errors.New("test error")

	mockCache := NewMockICache(ctrl)
	for _, ip := range ips {
		mockCache.EXPECT().hasKey(ip).Return(false).Times(retry)
	}

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Add(ips).Return(expectedErr).Times(retry)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: retry,
	}
	err := f.AddIPs(ips, r, m)
	assert.EqualError(t, err, expectedErr.Error())
}

func Test_DeleteIPs_IpNotDeletedFromCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl) // nolint:typecheck
	for _, ip := range ips {
		mockCache.EXPECT().deleteIP(ip, r, m).Return(false).Times(1)
	}

	f := &FilterManager{
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.DeleteIPs(ips, r, m)
	assert.NoError(t, err)
}

func Test_DeleteIPs_IPsDeletedFromCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4(), net.ParseIP("2.2.2.2").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl)
	for _, ip := range ips {
		mockCache.EXPECT().deleteIP(ip, r, m).Return(true).Times(1)
	}

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Delete(ips).Return(nil).Times(1)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.DeleteIPs(ips, r, m)
	assert.NoError(t, err)
}

func Test_DeleteIPs_HalfIPsDeletedFromCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4(), net.ParseIP("2.2.2.2").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}

	mockCache := NewMockICache(ctrl)
	// First IP is a cache hit.
	mockCache.EXPECT().deleteIP(ips[0], r, m).Return(true).Times(1)
	// Second IP is a cache miss.
	mockCache.EXPECT().deleteIP(ips[1], r, m).Return(false).Times(1)

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Delete([]net.IP{ips[0]}).Return(nil).Times(1)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: 1,
	}
	err := f.DeleteIPs(ips, r, m)
	assert.NoError(t, err)
}

func Test_DeleteIPs_Error(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4()}
	r := Requestor("test")
	m := RequestMetadata{RuleID: "rule-1"}
	expectedErr := errors.New("test error")
	retry := 3

	mockCache := NewMockICache(ctrl)
	for _, ip := range ips {
		mockCache.EXPECT().deleteIP(ip, r, m).Return(true).Times(retry)
		mockCache.EXPECT().addIP(ip, r, m).Times(retry)
	}

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Delete(ips).Return(expectedErr).Times(retry)

	f := &FilterManager{
		fm:    mockUpdateFn,
		c:     mockCache,
		l:     log.Logger().Named("filter-manager"),
		retry: retry,
	}
	err := f.DeleteIPs(ips, r, m)
	assert.EqualError(t, err, expectedErr.Error())
}

func Test_Reset(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ips := []net.IP{net.ParseIP("1.1.1.1").To4()}
	expectedErr := errors.New("test error")

	mockCache := NewMockICache(ctrl)
	mockCache.EXPECT().ips().Return(ips).Times(1)
	mockCache.EXPECT().reset().Times(1)

	mockUpdateFn := mocks.NewMockIFilterMap(ctrl)
	mockUpdateFn.EXPECT().Delete(ips).Return(nil).Times(1)

	f := &FilterManager{
		fm: mockUpdateFn,
		c:  mockCache,
		l:  log.Logger().Named("filter-manager"),
	}
	err := f.Reset()
	require.NoError(t, err)

	// Test filter-map error.
	mockCache.EXPECT().ips().Return(ips).Times(1)
	mockUpdateFn.EXPECT().Delete(ips).Return(expectedErr).Times(1)
	mockCache.EXPECT().reset().Times(0)
	err = f.Reset()
	assert.EqualError(t, err, expectedErr.Error())
}

func Test_Reset_EmptyCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCache := NewMockICache(ctrl)
	mockCache.EXPECT().ips().Return([]net.IP{}).Times(1)

	f := &FilterManager{
		c: mockCache,
		l: log.Logger().Named("filter-manager"),
	}
	err := f.Reset()
	assert.NoError(t, err)
}
