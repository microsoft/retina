// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filter

import (
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/filter/mocks"
	"github.com/stretchr/testify/assert"
)

func Test_mapKey(t *testing.T) {
	type args struct {
		ip net.IP
	}
	tests := []struct {
		name    string
		args    args
		want    filterMapKey //nolint:typecheck
		wantErr bool
	}{
		{
			name: "IPv4",
			args: args{
				ip: net.ParseIP("1.1.1.1").To4(),
			},
			want: filterMapKey{ //nolint:typecheck
				Prefixlen: uint32(32),
				Data:      uint32(16843009),
			},
			wantErr: false,
		},
		{
			name: "IPv6",
			args: args{
				ip: net.ParseIP("2001:db8::68").To16(),
			},
			want:    filterMapKey{}, //nolint:typecheck
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := mapKey(tt.args.ip)
			// If wantErr is true, we expect an err != nil.
			// If wantErr is false, we expect an err == nil.
			if (tt.wantErr && err == nil) || (!tt.wantErr && err != nil) {
				t.Errorf("mapKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, key)
		})
	}
}

func TestFilterMap_Add(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	input := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}
	expectedKeys := []filterMapKey{ //nolint:typecheck
		{
			Prefixlen: uint32(32),
			Data:      uint32(16843009),
		},
		{
			Prefixlen: uint32(32),
			Data:      uint32(33686018),
		},
	}
	expectedValues := []uint8{1, 1}

	mockKfm := mocks.NewMockIEbpfMap(ctrl)
	mockKfm.EXPECT().BatchUpdate(expectedKeys, expectedValues, gomock.Any()).Return(2, nil)

	f := &FilterMap{
		l:   log.Logger().Named("filter-map"),
		kfm: mockKfm,
	}
	err := f.Add(input)
	assert.NoError(t, err)

	// Test error case.
	mockKfm.EXPECT().BatchUpdate(expectedKeys, expectedValues, gomock.Any()).Return(0, errors.New("test error"))
	err = f.Add(input)
	assert.Error(t, err)
}

func TestFilterMap_Delete(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	input := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}
	expectedKeys := []filterMapKey{ //nolint:typecheck
		{
			Prefixlen: uint32(32),
			Data:      uint32(16843009),
		},
		{
			Prefixlen: uint32(32),
			Data:      uint32(33686018),
		},
	}

	mockKfm := mocks.NewMockIEbpfMap(ctrl)
	mockKfm.EXPECT().BatchDelete(expectedKeys, gomock.Any()).Return(2, nil)

	f := &FilterMap{
		l:   log.Logger().Named("filter-map"),
		kfm: mockKfm,
	}
	err := f.Delete(input)
	assert.NoError(t, err)

	// Test error case.
	mockKfm.EXPECT().BatchDelete(expectedKeys, gomock.Any()).Return(0, errors.New("test error"))
	err = f.Delete(input)
	assert.Error(t, err)
}

func TestFilterMap_Add_No_BatchApi(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	input := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}
	expectedKeys := []filterMapKey{ //nolint:typecheck
		{
			Prefixlen: uint32(32),
			Data:      uint32(16843009),
		},
		{
			Prefixlen: uint32(32),
			Data:      uint32(33686018),
		},
	}
	expectedValues := []uint8{1, 1}

	mockKfm := mocks.NewMockIEbpfMap(ctrl)
	// Test error case.
	mockKfm.EXPECT().BatchUpdate(expectedKeys, expectedValues, gomock.Any()).Return(0, errors.New("map batch api not supported (requires >= v5.6)"))
	mockKfm.EXPECT().Put(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	f := &FilterMap{
		l:   log.Logger().Named("filter-map"),
		kfm: mockKfm,
	}
	err := f.Add(input)
	assert.NoError(t, err)

	// Test error case.
	mockKfm.EXPECT().Put(gomock.Any(), gomock.Any()).Return(errors.New("test error"))
	err = f.Add(input)
	assert.Error(t, err)
}

func TestFilterMap_Delete_No_BatchApi(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	input := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}
	expectedKeys := []filterMapKey{ //nolint:typecheck
		{
			Prefixlen: uint32(32),
			Data:      uint32(16843009),
		},
		{
			Prefixlen: uint32(32),
			Data:      uint32(33686018),
		},
	}

	mockKfm := mocks.NewMockIEbpfMap(ctrl)
	mockKfm.EXPECT().BatchDelete(expectedKeys, gomock.Any()).Return(0, errors.New("map batch api not supported (requires >= v5.6)"))
	mockKfm.EXPECT().Delete(gomock.Any()).Return(nil).Times(2)

	f := &FilterMap{
		l:   log.Logger().Named("filter-map"),
		kfm: mockKfm,
	}
	err := f.Delete(input)
	assert.NoError(t, err)

	// Test error case.
	mockKfm.EXPECT().Delete(gomock.Any()).Return(errors.New("test error"))

	err = f.Delete(input)
	assert.Error(t, err)
}

func TestFilterMap_Close(t *testing.T) {
	f := &FilterMap{}
	f.Close() // expect no panic when obj is nil.

	f.obj = &filterObjects{} //nolint:typecheck
	f.Close()                // expect no panic when kfm is nil.
}
