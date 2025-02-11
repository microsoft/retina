package linuxutil

import (
	"errors"
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

var (
	MockGaugeVec   *metrics.MockGaugeVec
	MockCounterVec *metrics.MockCounterVec
)

var (
	errInterfaceNotSupported = errors.New("operation not supported")
	errOther                 = errors.New("other error")
)

func TestNewEthtool(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	opts := &EthtoolOpts{
		errOrDropKeysOnly: false,
		addZeroVal:        false,
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ethHandle := NewMockEthtoolInterface(ctrl)
	ethReader := NewEthtoolReader(opts, ethHandle)
	assert.NotNil(t, ethReader)
}

func TestNewEthtoolWithNil(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	opts := &EthtoolOpts{
		errOrDropKeysOnly: false,
		addZeroVal:        false,
	}

	ethReader := NewEthtoolReader(opts, nil)
	assert.NotNil(t, ethReader)
}

func TestReadInterfaceStats(t *testing.T) {
	testCache, err := lru.New[string, struct{}](10)
	if err != nil {
		t.Fatal("failed to create LRU cache: ", err)
	}
	globalLruCache = testCache

	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("ethtool test").Sugar()

	tests := []struct {
		name        string
		opts        *EthtoolOpts
		statsReturn map[string]uint64
		statErr     error
		result      map[string]uint64
		wantErr     bool
	}{
		{
			name: "test correct",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			},
			statsReturn: map[string]uint64{
				"rx_packets": 1,
			},
			statErr: nil,
			result: map[string]uint64{
				"rx_packets": 1,
			},
			wantErr: false,
		},
		{
			name: "test other error not added to cache",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			},
			statsReturn: nil,
			statErr:     errOther,
			result:      nil,
			wantErr:     true,
		},
		{
			name: "test unsupported interface",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			},
			statsReturn: nil,
			statErr:     errInterfaceNotSupported,
			result:      nil,
			wantErr:     false,
		},
		{
			name: "test skipped interface",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			},
			statsReturn: nil,
			statErr:     errInterfaceNotSupported,
			result:      nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		l.Infof("Running TestReadInterfaceStats %s", tt.name)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ethHandle := NewMockEthtoolInterface(ctrl)

		cachedEthHandle := NewCachedEthtool(ethHandle)

		ethReader := NewEthtoolReader(tt.opts, cachedEthHandle)

		assert.NotNil(t, ethReader)

		ethHandle.EXPECT().Stats(gomock.Any()).Return(tt.statsReturn, tt.statErr).AnyTimes()
		InitalizeMetricsForTesting(ctrl)

		if tt.statErr == nil {
			testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testmetric",
				Help: "testmetric",
			})

			MockGaugeVec.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).AnyTimes()
		}

		err := ethReader.readInterfaceStats()
		assert.Nil(t, err)

		if tt.statErr == nil {
			ethReader.updateMetrics()
		}

		if tt.statErr != nil && errors.Is(tt.statErr, errInterfaceNotSupported) {
			assert.NotNil(t, cachedEthHandle.unsupported, "cache should not be nil")
			assert.NotEqual(t, 0, cachedEthHandle.unsupported.Len(), "cache should contain interface")
		} else if tt.statErr != nil && !errors.Is(tt.statErr, errInterfaceNotSupported) {
			assert.Equal(t, 0, cachedEthHandle.unsupported.Len(), "cache should not add interface for other errors")
		}
	}
}

func InitalizeMetricsForTesting(ctrl *gomock.Controller) {
	metricsLogger := log.Logger().Named("metrics")
	metricsLogger.Info("Initializing metrics for testing")

	MockCounterVec = metrics.NewMockCounterVec(ctrl)
	MockGaugeVec = metrics.NewMockGaugeVec(ctrl)

	metrics.DropPacketsGauge = MockGaugeVec
	metrics.DropBytesGauge = MockGaugeVec
	metrics.ForwardBytesGauge = MockGaugeVec
	metrics.ForwardPacketsGauge = MockGaugeVec
	metrics.NodeConnectivityStatusGauge = MockGaugeVec
	metrics.NodeConnectivityLatencyGauge = MockGaugeVec
	metrics.TCPStateGauge = MockGaugeVec
	metrics.TCPConnectionRemoteGauge = MockGaugeVec
	metrics.TCPConnectionStatsGauge = MockGaugeVec
	metrics.IPConnectionStatsGauge = MockGaugeVec
	metrics.UDPConnectionStatsGauge = MockGaugeVec
	metrics.InterfaceStatsGauge = MockGaugeVec
	metrics.PluginManagerFailedToReconcileCounter = MockCounterVec
}
