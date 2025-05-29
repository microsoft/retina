// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	fm.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

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

func TestUpdateNamespaceLists(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	cfg, err := kcfg.GetConfig(testCfgFile)
	assert.NotNil(t, cfg)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := pubsub.NewMockPubSubInterface(ctrl)
	e := enricher.NewMockEnricherInterface(ctrl)
	fm := filtermanager.NewMockIFilterManager(ctrl)
	c := cache.NewMockCacheInterface(ctrl)
	c.EXPECT().GetIPsByNamespace(gomock.Any()).Return([]net.IP{}).AnyTimes()
	fm.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fm.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	me := InitModule(
		context.Background(),
		cfg,
		p,
		e,
		fm,
		c,
	)

	assert.NotNil(t, me)

	testcases := []struct {
		description            string
		namespaces             []string
		wantIncludedNamespaces map[string]struct{}
	}{
		{
			"input 0 namespaces",
			[]string{},
			map[string]struct{}{},
		},
		{
			"input 1 namespace (add)",
			[]string{"ns1"},
			map[string]struct{}{"ns1": {}},
		},
		{
			"input 1 namespace different than previous (add 1 & remove 1)",
			[]string{"ns2"},
			map[string]struct{}{"ns2": {}},
		},
		{
			"input 2 namespaces (add 1)",
			[]string{"ns1", "ns2"},
			map[string]struct{}{"ns1": {}, "ns2": {}},
		},
		{
			"input 2 namespaces different than previous 2 (add 2 & remove 2)",
			[]string{"ns3", "ns4"},
			map[string]struct{}{"ns3": {}, "ns4": {}},
		},
		{
			"input 0 namespaces (remove 2)",
			[]string{},
			map[string]struct{}{},
		},
	}

	for _, test := range testcases {
		t.Run(test.description, func(t *testing.T) {
			spec := (&api.MetricsSpec{}).
				WithIncludedNamespaces(test.namespaces)
			me.updateNamespaceLists(spec)
			assert.Equal(t, test.wantIncludedNamespaces, me.includedNamespaces)
		})
	}
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

func NewTestModule(
	l *log.ZapLogger,
	daemonConfig *kcfg.Config,
	registry map[string]AdvMetricsInterface,
	currentSpec *api.MetricsSpec,
) *Module {
	m := &Module{
		RWMutex:      &sync.RWMutex{},
		registry:     registry,
		l:            l,
		daemonConfig: daemonConfig,
		moduleCtx:    context.Background(),
		currentSpec:  currentSpec,
	}
	m.registryResetter = NewRegistryResetter(m)
	return m
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
			m:         NewTestModule(l, nil, make(map[string]AdvMetricsInterface), nil),
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
			m: NewTestModule(l, nil, map[string]AdvMetricsInterface{
				"drop_count":    testDropMetric,
				"forward_count": testForwardMetric,
			}, nil),
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
			m: NewTestModule(l, nil,
				map[string]AdvMetricsInterface{
					"drop_count":    testDropMetric,
					"drop_bytes":    testDropMetricBytes,
					"forward_count": testForwardMetric,
					"forward_bytes": testForwardMetricBytes,
				}, nil),
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
			m: NewTestModule(l, nil,
				map[string]AdvMetricsInterface{
					"drop_count":    testDropMetric,
					"drop_bytes":    testDropMetricBytes,
					"forward_count": testForwardMetric,
					"forward_bytes": testForwardMetricBytes,
				}, nil),
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
			m: NewTestModule(
				l, nil,
				map[string]AdvMetricsInterface{
					"drop_count": testDropMetric,
				},
				&api.MetricsSpec{
					ContextOptions: []api.MetricsContextOptions{
						{
							MetricName:        "drop_count",
							SourceLabels:      []string{"ip"},
							DestinationLabels: []string{"pod"},
							AdditionalLabels:  []string{"namespace"},
						},
					},
				},
			),
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
		tt.m.dirtyPods = common.NewDirtyCache()
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
			m: NewTestModule(l,
				&kcfg.Config{
					EnableAnnotations: true,
				}, nil, nil,
			),
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

type mockRegistryResetter struct {
	module         *Module
	mockMetricName string
	mockMetric     AdvMetricsInterface
}

func newMockRegistryResetter(m *Module, mockMetricName string, mockMetric AdvMetricsInterface) *mockRegistryResetter {
	return &mockRegistryResetter{
		module:         m,
		mockMetricName: mockMetricName,
		mockMetric:     mockMetric,
	}
}

func (mrr *mockRegistryResetter) Reset(_ []api.MetricsContextOptions) {
	mrr.module.registry[mrr.mockMetricName] = mrr.mockMetric

	mrr.mockMetric.Init(mrr.mockMetricName)
}

func TestModule_GenerateAdvMetrics(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	l := log.Logger().Named("test")

	disabledAnnotations := &kcfg.Config{
		EnableAnnotations: false,
	}
	enabledAnnotations := &kcfg.Config{
		EnableAnnotations: true,
	}

	IP1 := "10.0.0.1"
	IP2 := "10.0.0.2"
	IP3 := "10.0.0.3"
	IP4 := "10.0.0.4"

	newFlow := func(src, dst string) *flow.Flow {
		return utils.ToFlow(
			l,
			int64(0),
			net.ParseIP(src),
			net.ParseIP(dst),
			uint32(8080),
			uint32(8080),
			uint8(6),
			2, // Direction: Ingress
			utils.Verdict_DNS,
		)
	}
	newEvent := func(f *flow.Flow) *v1.Event {
		return &v1.Event{
			Event:     f,
			Timestamp: f.GetTime(),
		}
	}

	ip1ToIP2 := newFlow(IP1, IP2)
	ip2ToIP1 := newFlow(IP2, IP1)
	ip1ToIP3 := newFlow(IP1, IP3)
	ip1ToIP4 := newFlow(IP1, IP4)
	ip4ToIP1 := newFlow(IP4, IP1)

	tests := []struct {
		name           string
		config         *kcfg.Config
		IPsInCache     []net.IP
		generatedFlows []*flow.Flow
		processedFlows []*flow.Flow
	}{
		{
			name:           "Disabled Annotations",
			config:         disabledAnnotations,
			IPsInCache:     []net.IP{},
			generatedFlows: []*flow.Flow{ip1ToIP2, ip2ToIP1},
			processedFlows: []*flow.Flow{ip1ToIP2, ip2ToIP1},
		},
		{
			name:           "Enabled Annotations - no ip in cache",
			config:         enabledAnnotations,
			IPsInCache:     []net.IP{},
			generatedFlows: []*flow.Flow{ip1ToIP2, ip2ToIP1},
			processedFlows: []*flow.Flow{},
		},
		{
			name:           "Enabled Annotations - ip in cache, flow ip not in cache",
			config:         enabledAnnotations,
			IPsInCache:     []net.IP{net.ParseIP(IP3)},
			generatedFlows: []*flow.Flow{ip1ToIP2, ip2ToIP1},
			processedFlows: []*flow.Flow{},
		},
		{
			name:           "Enabled Annotations - 1 ip in cache, flow ip in cache",
			config:         enabledAnnotations,
			IPsInCache:     []net.IP{net.ParseIP(IP3)},
			generatedFlows: []*flow.Flow{ip1ToIP3, ip2ToIP1},
			processedFlows: []*flow.Flow{ip1ToIP3},
		},
		{
			name:           "Enabled Annotations - 2 ip in cache, flows with and without ip in cache",
			config:         enabledAnnotations,
			IPsInCache:     []net.IP{net.ParseIP(IP2), net.ParseIP(IP3)},
			generatedFlows: []*flow.Flow{ip1ToIP3, ip2ToIP1, ip1ToIP4, ip4ToIP1},
			processedFlows: []*flow.Flow{ip2ToIP1, ip1ToIP3},
		},
	}

	for _, tt := range tests {

		ctrl := gomock.NewController(t)
		mockMetricName := "mock"
		myMockMetric := NewMockAdvMetricsInterface(ctrl) //nolint:typecheck // gomock

		m := &Module{
			wg:           sync.WaitGroup{},
			RWMutex:      &sync.RWMutex{},
			registry:     map[string]AdvMetricsInterface{},
			moduleCtx:    context.TODO(),
			daemonConfig: tt.config,
			l:            l,
			daemonCache:  cache.NewMockCacheInterface(ctrl),
		}
		m.registryResetter = newMockRegistryResetter(m, mockMetricName, myMockMetric)

		log.Logger().Info("***** Running test *****", zap.String("name", tt.name))

		p := pubsub.NewMockPubSubInterface(ctrl)        //nolint:typecheck // gomock
		e := enricher.NewMockEnricherInterface(ctrl)    //nolint:typecheck // gomock
		fm := filtermanager.NewMockIFilterManager(ctrl) //nolint:typecheck // gomock
		c := cache.NewMockCacheInterface(ctrl)          //nolint:typecheck // gomock
		m.pubsub = p
		m.enricher = e
		m.filterManager = fm
		m.daemonCache = c
		m.dirtyPods = common.NewDirtyCache()

		testRing := container.NewRing(container.Capacity7)
		testRingReader := container.NewRingReader(testRing, testRing.OldestWrite())

		fm.EXPECT().HasIP(gomock.Any()).DoAndReturn(func(ip net.IP) bool {
			for _, cachedIP := range tt.IPsInCache {
				if ip.Equal(cachedIP) {
					return true
				}
			}
			return false
		}).AnyTimes()

		p.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return("test").AnyTimes()
		e.EXPECT().ExportReader().Return(testRingReader).MinTimes(1)
		myMockMetric.EXPECT().Init(mockMetricName).AnyTimes()
		for _, f := range tt.processedFlows {
			myMockMetric.EXPECT().ProcessFlow(f).Times(1)
		}

		spec := (&api.MetricsSpec{}).
			WithIncludedNamespaces([]string{}).
			WithMetricsContextOptions([]string{mockMetricName}, []string{"ip"}, []string{"pod"})

		err := m.Reconcile(spec)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for _, f := range tt.generatedFlows {
			testRing.Write(newEvent(f))
		}
		// for some reason, the n-th element is read when n+1-th element is written
		dummyFlow := newFlow("127.0.0.1", "127.0.0.2")
		testRing.Write(newEvent(dummyFlow))

		time.Sleep(1 * time.Second) // wait for goroutines to react to event
	}
}
