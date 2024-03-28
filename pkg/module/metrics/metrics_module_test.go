// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/cilium/cilium/pkg/hubble/container"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

const (
	testCfgFile = "../../config/testwith/config.yaml"
)

func TestAppendIncludeList(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg, err := kcfg.GetConfig(testCfgFile)
	assert.NotNil(t, cfg)
	assert.Nil(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := pubsub.NewMockPubSubInterface(ctrl)        //nolint:typecheck
	e := enricher.NewMockEnricherInterface(ctrl)    //nolint:typecheck
	fm := filtermanager.NewMockIFilterManager(ctrl) //nolint:typecheck
	c := cache.NewMockCacheInterface(ctrl)          //nolint:typecheck
	c.EXPECT().GetIPsByNamespace(gomock.Any()).Return([]net.IP{}).AnyTimes()
	fm.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	me := InitModule(
		context.Background(),
		cfg,
		p,
		e,
		fm,
		c,
	)
	assert.NotNil(t, me)

	me.appendIncludeList([]string{"test"})
}

func TestPodCallBack(t *testing.T) {
	cfg, err := kcfg.GetConfig(testCfgFile)
	cfg.EnableAnnotations = true
	assert.NotNil(t, cfg)
	assert.Nil(t, err)
	log.SetupZapLogger(log.GetDefaultLogOpts())

	tests := []struct {
		name           string
		addObjs        []*common.RetinaEndpoint
		deleteObjs     []*common.RetinaEndpoint
		includeNslist  []string
		excludeNslist  []string
		deleteExpected []net.IP
		addExpected    []net.IP
		fmHasIP        bool
	}{
		{
			name: "test",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
			},
			includeNslist:  []string{"ns1"},
			deleteExpected: nil,
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
		},
		{
			name: "test2",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod2", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
			},
			includeNslist:  []string{"ns1"},
			deleteExpected: nil,
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
		},
		{
			name: "test3",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod2", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
			},
			deleteObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 2)}),
			},
			includeNslist:  []string{"ns1"},
			deleteExpected: []net.IP{net.IPv4(10, 0, 0, 2)},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
		},
		{
			name: "test4",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod2", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod2", "ns3", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 4)}),
			},
			deleteObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 2)}),
			},
			includeNslist:  []string{"ns1"},
			deleteExpected: []net.IP{net.IPv4(10, 0, 0, 2)},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
		},
		{
			name: "pod of interest namespace not of interest",
			addObjs: []*common.RetinaEndpoint{
				func() *common.RetinaEndpoint {
					ep := common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)})
					ep.SetAnnotations(map[string]string{common.RetinaPodAnnotation: common.RetinaPodAnnotationValue})
					return ep
				}(),
			},
			includeNslist:  []string{"ns2"},
			deleteExpected: []net.IP{},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
		},
		{
			name: "pod not of interest and 1 namespace of interest",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod1", "ns2", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 2)}),
			},
			includeNslist:  []string{"ns2"},
			deleteExpected: []net.IP{},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 2)},
		},
		{
			name: "ns not of interest, pod not annotated, pod ip in filtermap",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
				common.NewRetinaEndpoint("pod1", "ns2", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 2)}),
			},
			includeNslist:  []string{"ns2"},
			deleteExpected: []net.IP{net.IPv4(10, 0, 0, 1)},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 2)},
			fmHasIP:        true,
		},
		{
			name: "pod and ns of interest",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
			},
			includeNslist:  []string{"ns1"},
			deleteExpected: []net.IP{},
			addExpected:    []net.IP{net.IPv4(10, 0, 0, 1)},
			fmHasIP:        true,
		},
		{
			name: "pod and ns not of interest, but fm has ip (annotation removed)",
			addObjs: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)}),
			},
			includeNslist:  []string{"ns2"},
			deleteExpected: []net.IP{net.IPv4(10, 0, 0, 1)},
			addExpected:    []net.IP{},
			fmHasIP:        true,
		},
	}

	for _, tt := range tests {
		log.Logger().Info("***** Running test *****", zap.String("name", tt.name))
		ctrl := gomock.NewController(t)

		p := pubsub.NewMockPubSubInterface(ctrl)        //nolint:typecheck
		e := enricher.NewMockEnricherInterface(ctrl)    //nolint:typecheck
		fm := filtermanager.NewMockIFilterManager(ctrl) //nolint:typecheck
		c := cache.NewMockCacheInterface(ctrl)          //nolint:typecheck
		c.EXPECT().GetIPsByNamespace(gomock.Any()).Return([]net.IP{}).AnyTimes()
		fm.EXPECT().AddIPs([]net.IP{}, gomock.Any(), gomock.Any()).Return(nil).Times(1)
		fm.EXPECT().HasIP(gomock.Any()).Return(tt.fmHasIP).AnyTimes()
		if len(tt.addExpected) > 0 {
			fm.EXPECT().AddIPs(tt.addExpected, gomock.Any(), gomock.Any()).Return(nil).Times(1)
		}
		if len(tt.deleteExpected) > 0 {
			fm.EXPECT().DeleteIPs(tt.deleteExpected, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		}

		me := &Module{
			RWMutex:       &sync.RWMutex{},
			l:             log.Logger().Named(string("MetricModule")),
			pubsub:        p,
			configs:       make([]*api.MetricsConfiguration, 0),
			enricher:      e,
			wg:            sync.WaitGroup{},
			registry:      make(map[string]AdvMetricsInterface),
			moduleCtx:     context.Background(),
			filterManager: fm,
			daemonCache:   c,
			dirtyPods:     common.NewDirtyCache(),
			pubsubPodSub:  "",
			daemonConfig:  cfg,
		}
		assert.NotNil(t, me)
		me.appendIncludeList(tt.includeNslist)
		for _, obj := range tt.addObjs {
			ev := cache.NewCacheEvent(cache.EventTypePodAdded, obj)
			me.PodCallBackFn(ev)
		}

		for _, obj := range tt.deleteObjs {
			ev := cache.NewCacheEvent(cache.EventTypePodDeleted, obj)
			me.PodCallBackFn(ev)
		}

		me.applyDirtyPods()

		ctrl.Finish()
	}
}

// Deletes pod ips for specific namespace, but confirms that delete is called for namespace request metadata
func TestModule_NamespaceAndPodUpdates(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	cfg, _ := kcfg.GetConfig(testCfgFile)
	cfg.EnableAnnotations = true
	p := pubsub.NewMockPubSubInterface(ctrl)        //nolint:typecheck
	e := enricher.NewMockEnricherInterface(ctrl)    //nolint:typecheck
	fm := filtermanager.NewMockIFilterManager(ctrl) //nolint:typecheck
	c := cache.NewMockCacheInterface(ctrl)          //nolint:typecheck
	ns1 := "ns1"
	ns2 := "ns2"

	includedNs := make(map[string]struct{})
	includedNs[ns1] = struct{}{}

	// adding new namespace with new pod ips
	c.EXPECT().GetIPsByNamespace(ns2).Times(1).Return([]net.IP{net.IPv4(10, 0, 0, 2)})
	fm.EXPECT().AddIPs([]net.IP{net.IPv4(10, 0, 0, 2)}, gomock.Any(), moduleReqMetadata).Times(1)
	c.EXPECT().GetIPsByNamespace(ns1).Times(1).Return([]net.IP{net.IPv4(10, 0, 0, 1)})
	fm.EXPECT().HasIP(gomock.Any()).Return(true).AnyTimes()

	// confirm specific calls to filter manager and cache with correct request metadata
	fm.EXPECT().DeleteIPs([]net.IP{net.IPv4(10, 0, 0, 1)}, gomock.Any(), moduleReqMetadata).Times(1)
	// fm.EXPECT().DeleteIPs([]net.IP{net.IPv4(10, 0, 0, 2)}, gomock.Any(), modulePodReqMetadata).Times(1)

	me := &Module{
		RWMutex:            &sync.RWMutex{},
		l:                  log.Logger().Named(string("MetricModule")),
		pubsub:             p,
		configs:            make([]*api.MetricsConfiguration, 0),
		enricher:           e,
		wg:                 sync.WaitGroup{},
		registry:           make(map[string]AdvMetricsInterface),
		moduleCtx:          context.Background(),
		filterManager:      fm,
		daemonCache:        c,
		dirtyPods:          common.NewDirtyCache(),
		pubsubPodSub:       "",
		daemonConfig:       cfg,
		includedNamespaces: includedNs,
	}
	podobj := common.NewRetinaEndpoint("pod1", ns1, &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 1)})
	podobj.SetAnnotations(map[string]string{common.RetinaPodAnnotation: common.RetinaPodAnnotationValue})
	podobj2 := common.NewRetinaEndpoint("pod2", ns2, &common.IPAddresses{IPv4: net.IPv4(10, 0, 0, 2)})
	podobj2.SetAnnotations(map[string]string{common.RetinaPodAnnotation: common.RetinaPodAnnotationValue})

	ev := cache.NewCacheEvent(cache.EventTypePodAdded, podobj)
	ev2 := cache.NewCacheEvent(cache.EventTypePodAdded, podobj2)

	// add pod1 and pod2 to fm (annotations)
	me.PodCallBackFn(ev)
	me.PodCallBackFn(ev2)

	// add ns2 to include list, but also remove ns1 from fm
	me.appendIncludeList([]string{ns2})

	// remove annotation
	podobj2.SetAnnotations(map[string]string{})
	ev3 := cache.NewCacheEvent(cache.EventTypePodAdded, podobj2)
	me.PodCallBackFn(ev3)

	ctrl.Finish()
}

func TestModule_Reconcile(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	l := log.Logger().Named("test")

	testDropMetric := &DropCountMetrics{
		baseMetricObject: baseMetricObject{
			advEnable: true,
			ctxOptions: &api.MetricsContextOptions{
				MetricName:        "drop_count",
				SourceLabels:      []string{"ip"},
				DestinationLabels: []string{"pod"},
				AdditionalLabels:  []string{"namespace"},
			},
		},
	}
	testDropMetric.Init("drop_count")
	testDropMetricBytes := &DropCountMetrics{
		baseMetricObject: baseMetricObject{
			advEnable: true,
			ctxOptions: &api.MetricsContextOptions{
				MetricName:   "drop_bytes",
				SourceLabels: []string{"ip"},
			},
		},
	}
	testDropMetricBytes.Init("drop_bytes")
	testForwardMetric := &ForwardMetrics{
		baseMetricObject: baseMetricObject{
			advEnable: true,
			ctxOptions: &api.MetricsContextOptions{
				MetricName:        "forward_count",
				SourceLabels:      []string{"ip"},
				DestinationLabels: []string{"pod"},
				AdditionalLabels:  []string{"namespace"},
			},
		},
	}
	testForwardMetric.Init("forward_count")
	testForwardMetricBytes := &ForwardMetrics{
		baseMetricObject: baseMetricObject{
			advEnable: true,
			ctxOptions: &api.MetricsContextOptions{
				MetricName:   "forward_bytes",
				SourceLabels: []string{"ip"},
			},
		},
	}
	testForwardMetricBytes.Init("forward_bytes")

	tests := []struct {
		name          string
		spec          *api.MetricsSpec
		m             *Module
		expectErr     bool
		expectNoCalls bool
	}{
		{
			name: "Registry is empty and no error",
			spec: &api.MetricsSpec{
				ContextOptions: []api.MetricsContextOptions{
					{
						MetricName:        "drop_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
				},
			},
			m: &Module{
				RWMutex:   &sync.RWMutex{},
				registry:  make(map[string]AdvMetricsInterface),
				l:         l,
				moduleCtx: context.Background(),
			},
			expectErr: false,
		},
		{
			name: "Registry is not empty and no error",
			spec: &api.MetricsSpec{
				ContextOptions: []api.MetricsContextOptions{
					{
						MetricName:        "drop_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
					{
						MetricName:        "forward_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
				},
			},
			m: &Module{
				RWMutex: &sync.RWMutex{},
				registry: map[string]AdvMetricsInterface{
					"drop_count":    testDropMetric,
					"forward_count": testForwardMetric,
				},
				moduleCtx: context.Background(),
				l:         l,
			},
			expectErr: false,
		},
		{
			name: "Registry is not empty and no error for bytes",
			spec: &api.MetricsSpec{
				ContextOptions: []api.MetricsContextOptions{
					{
						MetricName:        "drop_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
					{
						MetricName:   "drop_bytes",
						SourceLabels: []string{"ip"},
					},
					{
						MetricName:        "forward_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
					{
						MetricName:   "forward_bytes",
						SourceLabels: []string{"ip"},
					},
				},
			},
			m: &Module{
				RWMutex: &sync.RWMutex{},
				registry: map[string]AdvMetricsInterface{
					"drop_count":    testDropMetric,
					"drop_bytes":    testDropMetricBytes,
					"forward_count": testForwardMetric,
					"forward_bytes": testForwardMetricBytes,
				},
				moduleCtx: context.Background(),
				l:         l,
			},
			expectErr: false,
		},
		{
			name: "Registry is not empty and error for invalid name",
			spec: &api.MetricsSpec{
				ContextOptions: []api.MetricsContextOptions{
					{
						MetricName:        "drop_hello",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
					{
						MetricName:   "drop_bytes",
						SourceLabels: []string{"ip"},
					},
					{
						MetricName:        "forward_count",
						SourceLabels:      []string{"ip", "pod"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
					{
						MetricName:   "forward_hi",
						SourceLabels: []string{"ip"},
					},
				},
			},
			m: &Module{
				RWMutex: &sync.RWMutex{},
				registry: map[string]AdvMetricsInterface{
					"drop_count":    testDropMetric,
					"drop_bytes":    testDropMetricBytes,
					"forward_count": testForwardMetric,
					"forward_bytes": testForwardMetricBytes,
				},
				moduleCtx: context.Background(),
				l:         l,
			},
			expectErr: true,
		},
		{
			name: "Expect no change for spec the same",
			spec: &api.MetricsSpec{
				ContextOptions: []api.MetricsContextOptions{
					{
						MetricName:        "drop_count",
						SourceLabels:      []string{"ip"},
						DestinationLabels: []string{"pod"},
						AdditionalLabels:  []string{"namespace"},
					},
				},
			},
			m: &Module{
				RWMutex: &sync.RWMutex{},
				registry: map[string]AdvMetricsInterface{
					"drop_count": testDropMetric,
				},
				moduleCtx: context.Background(),
				l:         l,
				currentSpec: &api.MetricsSpec{
					ContextOptions: []api.MetricsContextOptions{
						{
							MetricName:        "drop_count",
							SourceLabels:      []string{"ip"},
							DestinationLabels: []string{"pod"},
							AdditionalLabels:  []string{"namespace"},
						},
					},
				},
			},
			expectErr:     false,
			expectNoCalls: true,
		},
	}

	for _, tt := range tests {
		log.Logger().Info("***** Running test *****", zap.String("name", tt.name))
		p := pubsub.NewMockPubSubInterface(ctrl)        //nolint:typecheck
		e := enricher.NewMockEnricherInterface(ctrl)    //nolint:typecheck
		fm := filtermanager.NewMockIFilterManager(ctrl) //nolint:typecheck
		c := cache.NewMockCacheInterface(ctrl)          //nolint:typecheck
		tt.m.pubsub = p
		tt.m.enricher = e
		tt.m.filterManager = fm
		tt.m.daemonCache = c
		testRing := container.NewRing(container.Capacity1)
		testRingReader := container.NewRingReader(testRing, 0)
		p.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return("test").AnyTimes()
		e.EXPECT().ExportReader().AnyTimes().Return(testRingReader)

		if tt.expectNoCalls {
			p.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(0)
			e.EXPECT().ExportReader().Times(0)
		}

		err := tt.m.Reconcile(tt.spec)
		if err != nil && !tt.expectErr {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestPodAnnotated(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")
	// test podAnnotated function
	tests := []struct {
		name        string
		annotations map[string]string
		m           *Module
		expected    bool
	}{
		{
			name: "pod annotated disabled annotations",
			annotations: map[string]string{
				common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
			},
			m:        &Module{},
			expected: false,
		},
		{
			name: "pod annotated",
			annotations: map[string]string{
				common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
			},
			m: &Module{
				daemonConfig: &kcfg.Config{
					EnableAnnotations: true,
				},
				l: l,
			},
			expected: true,
		},
		{
			name: "pod not annotated",
			annotations: map[string]string{
				common.RetinaPodAnnotation: "test",
			},
			m:        &Module{},
			expected: false,
		},
	}
	for _, tt := range tests {
		l.Info("***** Running test *****", zap.String("name", tt.name))
		assert.Equal(t, tt.expected, tt.m.podAnnotated(tt.annotations))
	}
}
