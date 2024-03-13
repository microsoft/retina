// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"fmt"
	"net"
	"testing"

	"github.com/cakturk/go-netstat/netstat"
	gomock "github.com/golang/mock/gomock"
	"github.com/microsoft/retina/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewNetstatReader(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	opts := &NetstatOpts{
		CuratedKeys: false,
		AddZeroVal:  false,
		ListenSock:  false,
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ns := NewMockNetstatInterface(ctrl)
	nr := NewNetstatReader(opts, ns)
	assert.NotNil(t, nr)
}

func TestReadConnStats(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	opts := &NetstatOpts{
		CuratedKeys: false,
		AddZeroVal:  false,
		ListenSock:  false,
	}
	tests := []struct {
		name        string
		filePath    string
		result      *ConnectionStats
		totalsCheck map[string]int
		checkVals   bool
		wantErr     bool
		addValZero  bool
	}{
		{
			name:     "test correct",
			filePath: "testdata/correct-netstat",
			totalsCheck: map[string]int{
				"TcpExt":   55,
				"IpExt":    5,
				"MPTcpExt": 0,
			},
			wantErr: false,
		},
		{
			name:     "test correct with zero values",
			filePath: "testdata/correct-netstat",
			totalsCheck: map[string]int{
				"TcpExt":   123,
				"IpExt":    18,
				"MPTcpExt": 0,
			},
			wantErr:    false,
			addValZero: true,
		},
		{
			name:     "test some correct with zero values",
			filePath: "testdata/somecorrect-netstat1",
			result: &ConnectionStats{
				TcpExt: nil,
				IpExt: map[string]uint64{
					"InNoRoutes":      18965,
					"InTruncatedPkts": 0,
				},
				MPTcpExt: map[string]uint64{
					"Test1": 10,
				},
			},
			wantErr:    false,
			checkVals:  true,
			addValZero: true,
		},
		{
			name:     "test some correct with nonzero",
			filePath: "testdata/somecorrect-netstat1",
			result: &ConnectionStats{
				TcpExt: nil,
				IpExt: map[string]uint64{
					"InNoRoutes": 18965,
				},
				MPTcpExt: map[string]uint64{
					"Test1": 10,
				},
			},
			wantErr:   false,
			checkVals: true,
		},
		{
			name:     "test wrong",
			filePath: "testdata/wrong-netstat",
			result: &ConnectionStats{
				TcpExt: map[string]uint64{},
				IpExt:  map[string]uint64{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.addValZero {
				opts.AddZeroVal = true
			} else {
				opts.AddZeroVal = false
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ns := NewMockNetstatInterface(ctrl)
			nr := NewNetstatReader(opts, ns)
			InitalizeMetricsForTesting(ctrl)

			testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testmetric",
				Help: "testmetric",
			})

			MockGaugeVec.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).AnyTimes()

			assert.NotNil(t, nr)
			err := nr.readConnectionStats(tt.filePath)
			if tt.wantErr {
				assert.NotNil(t, err, "Expected error but got nil")
			} else {
				assert.Nil(t, err, "Expected nil but got err")
				assert.NotNil(t, nr.connStats, "Expected data got nil")
				if tt.checkVals {
					assert.Equal(t, tt.result, nr.connStats, "Expected data got nil")
				} else {
					assert.Equal(t, len(nr.connStats.TcpExt), tt.totalsCheck["TcpExt"], "Read values are not equal to expected")
					assert.Equal(t, len(nr.connStats.IpExt), tt.totalsCheck["IpExt"], "Read values are not equal to expected")
					assert.Equal(t, len(nr.connStats.MPTcpExt), tt.totalsCheck["MPTcpExt"], "Read values are not equal to expected")
				}

				nr.updateMetrics()
			}
		})
	}
}

func TestProcessSocks(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tests := []struct {
		name    string
		data    []netstat.SockTabEntry
		result  *SocketStats
		wantErr bool
	}{
		{
			name: "localhost",
			data: []netstat.SockTabEntry{
				{
					LocalAddr: &netstat.SockAddr{
						IP:   net.IPv4(127, 0, 0, 1).To4(),
						Port: 80,
					},
					RemoteAddr: &netstat.SockAddr{
						IP:   net.IPv4(127, 0, 0, 1).To4(),
						Port: 80,
					},
					State: 1,
					UID:   0,
				},
			},
			result: &SocketStats{
				totalActiveSockets: 1,
				socketByState: map[string]int{
					"ESTABLISHED": 1,
				},
				socketByRemoteAddr: map[string]int{
					"127.0.0.1:80": 1,
				},
			},
			wantErr: false,
		},
		{
			name: "localhost2",
			data: []netstat.SockTabEntry{
				{
					LocalAddr: &netstat.SockAddr{
						IP:   net.IPv4(127, 0, 0, 1).To4(),
						Port: 80,
					},
					RemoteAddr: &netstat.SockAddr{
						IP:   net.IPv4(127, 0, 0, 1).To4(),
						Port: 80,
					},
					State: 1,
				},
				{
					RemoteAddr: &netstat.SockAddr{
						IP:   net.IPv4(127, 0, 0, 1).To4(),
						Port: 443,
					},
					State: 1,
				},
			},
			result: &SocketStats{
				totalActiveSockets: 2,
				socketByState: map[string]int{
					"ESTABLISHED": 2,
				},
				socketByRemoteAddr: map[string]int{
					"127.0.0.1:80":  1,
					"127.0.0.1:443": 1,
				},
			},
			wantErr: false,
		},
		{
			name: "nil",
			data: nil,
			result: &SocketStats{
				totalActiveSockets: 0,
				socketByState:      map[string]int{},
				socketByRemoteAddr: map[string]int{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		retData := processSocks(tt.data)
		assert.NotNil(t, retData)
		assert.Equal(t, tt.result.totalActiveSockets, retData.totalActiveSockets)
		assert.Equal(t, tt.result.socketByState, retData.socketByState)
		assert.Equal(t, tt.result.socketByRemoteAddr, retData.socketByRemoteAddr)
	}
}

func TestReadSockStatsError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	opts := &NetstatOpts{
		CuratedKeys: false,
		AddZeroVal:  false,
		ListenSock:  false,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ns := NewMockNetstatInterface(ctrl)
	nr := NewNetstatReader(opts, ns)
	InitalizeMetricsForTesting(ctrl)

	testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "testmetric",
		Help: "testmetric",
	})

	MockGaugeVec.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).AnyTimes()
	ns.EXPECT().UDPSocks(gomock.Any()).Return(nil, fmt.Errorf("Random error")).Times(1)

	assert.NotNil(t, nr)
	err := nr.readSockStats()
	assert.NotNil(t, err, "Expected error but got nil tetsname")
}

func TestReadSockStats(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	opts := &NetstatOpts{
		CuratedKeys: false,
		AddZeroVal:  false,
		ListenSock:  false,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ns := NewMockNetstatInterface(ctrl)
	nr := NewNetstatReader(opts, ns)
	InitalizeMetricsForTesting(ctrl)

	testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "testmetric",
		Help: "testmetric",
	})

	MockGaugeVec.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).AnyTimes()
	ns.EXPECT().UDPSocks(gomock.Any()).Return([]netstat.SockTabEntry{
		{
			LocalAddr: &netstat.SockAddr{
				IP:   net.IPv4(127, 0, 0, 1).To4(),
				Port: 80,
			},
			RemoteAddr: &netstat.SockAddr{
				IP:   net.IPv4(127, 0, 0, 1).To4(),
				Port: 80,
			},
			State: 1,
			UID:   0,
		},
	}, nil).Times(1)

	ns.EXPECT().TCPSocks(gomock.Any()).Return([]netstat.SockTabEntry{
		{
			LocalAddr: &netstat.SockAddr{
				IP:   net.IPv4(127, 0, 0, 1).To4(),
				Port: 80,
			},
			RemoteAddr: &netstat.SockAddr{
				IP:   net.IPv4(127, 0, 0, 1).To4(),
				Port: 80,
			},
			State: 1,
			UID:   0,
		},
	}, nil).Times(1)

	assert.NotNil(t, nr)
	err := nr.readSockStats()
	assert.Nil(t, err, "Expected nil but got err tetsname")
	assert.NotNil(t, nr.connStats, "Expected data got nil tetsname")
	assert.Equal(t, nr.connStats.UdpSockets.totalActiveSockets, 1, "Read values are not equal to expected tetsname")
	assert.Equal(t, nr.connStats.UdpSockets.socketByState["ESTABLISHED"], 1, "Read values are not equal to expected tetsname")

	assert.Equal(t, nr.connStats.TcpSockets.totalActiveSockets, 1, "Read values are not equal to expected tetsname")
	assert.Equal(t, nr.connStats.TcpSockets.socketByState["ESTABLISHED"], 1, "Read values are not equal to expected tetsname")

	nr.updateMetrics()
}
