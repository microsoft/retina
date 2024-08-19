package linuxutil

import (
	"errors"
	"fmt"
	"testing"

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

var globalUnsupportedCache = make(map[string]bool)

func TestReadInterfaceStats(t *testing.T) {
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
			name: "test unsported interface",
			opts: &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			},
			statsReturn: nil,
			statErr:     errors.New("interface not supported"),
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
			statErr:     errors.New("interface not supported"),
			result:      nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		l.Infof("Running TestReadInterfaceStats %s", tt.name)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ethHandle := NewMockEthtoolInterface(ctrl)
		ethReader := NewEthtoolReader(tt.opts, ethHandle)

		ethReader.unsupported = globalUnsupportedCache

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

		if tt.statErr != nil && tt.statErr.Error() == "interface not supported" {
			assert.True(t, ethReader.unsupported != nil, "unsupported map should not be nil")
			assert.True(t, len(ethReader.unsupported) > 0, "unsupported map should contain interfaces")
		}

		globalUnsupportedCache = ethReader.unsupported
		fmt.Println("Current unsupported cache: ", ethReader.unsupported, "Global unsupported cache: ", globalUnsupportedCache)

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
