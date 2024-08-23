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
	MockGaugeVec   *metrics.MockIGaugeVec
	MockCounterVec *metrics.MockICounterVec
)
var errInterfaceNotSupported = errors.New("interface not supported")

func TestNewEthtool(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	opts := &EthtoolOpts{
		errOrDropKeysOnly: false,
		addZeroVal:        false,
		limit:             10,
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ethHandle := NewMockEthtoolInterface(ctrl)
	// cachedEthHandle := NewCachedEthtool(ethHandle, opts)
	ethReader := NewEthtoolReader(opts, ethHandle)
	assert.NotNil(t, ethReader)
}

func TestNewEthtoolWithNil(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	opts := &EthtoolOpts{
		errOrDropKeysOnly: false,
		addZeroVal:        false,
		limit:             10,
	}

	ethReader := NewEthtoolReader(opts, nil)
	assert.NotNil(t, ethReader)
}

func TestReadInterfaceStats(t *testing.T) {
	globalCache, _ := lru.New[string, struct{}](10)

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
				limit:             10,
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
			name: "test unsported interface",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
				limit:             10,
			},
			statsReturn: nil,
			statErr:     errInterfaceNotSupported,

			result:  nil,
			wantErr: false,
		},
		{
			name: "test skipped interface",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
				limit:             10,
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

		cachedEthHandle := NewCachedEthtool(ethHandle, tt.opts)
		cachedEthHandle.unsupported = globalCache

		ethReader := NewEthtoolReader(tt.opts, cachedEthHandle)

		assert.NotNil(t, ethReader)

		ethHandle.EXPECT().Stats(gomock.Any()).Return(tt.statsReturn, tt.statErr).AnyTimes()
		ethHandle.EXPECT().Close().Times(1)
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
		}

		globalCache = cachedEthHandle.unsupported
	}
}

func InitalizeMetricsForTesting(ctrl *gomock.Controller) {
	metricsLogger := log.Logger().Named("metrics")
	metricsLogger.Info("Initializing metrics for testing")

	MockCounterVec = metrics.NewMockICounterVec(ctrl)
	MockGaugeVec = metrics.NewMockIGaugeVec(ctrl) //nolint:typecheck

	metrics.DropCounter = MockGaugeVec
	metrics.DropBytesCounter = MockGaugeVec
	metrics.ForwardBytesCounter = MockGaugeVec
	metrics.ForwardCounter = MockGaugeVec
	metrics.NodeConnectivityStatusGauge = MockGaugeVec
	metrics.NodeConnectivityLatencyGauge = MockGaugeVec
	metrics.TCPStateGauge = MockGaugeVec
	metrics.TCPConnectionRemoteGauge = MockGaugeVec
	metrics.TCPConnectionStats = MockGaugeVec
	metrics.TCPFlagCounters = MockGaugeVec
	metrics.IPConnectionStats = MockGaugeVec
	metrics.UDPConnectionStats = MockGaugeVec
	metrics.UDPActiveSocketsCounter = MockGaugeVec
	metrics.InterfaceStats = MockGaugeVec
	metrics.PluginManagerFailedToReconcileCounter = MockCounterVec
}
