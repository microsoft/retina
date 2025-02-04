package scaletest

import (
	"context"
	"time"
)

// Options holds parameters for the scale test
type Options struct {
	Ctx                           context.Context
	Namespace                     string
	MaxKwokPodsPerNode            int
	NumKwokDeployments            int
	NumKwokReplicas               int
	MaxRealPodsPerNode            int
	NumRealDeployments            int
	RealPodType                   string
	NumRealReplicas               int
	NumRealServices               int
	NumNetworkPolicies            int
	NumUnapliedNetworkPolicies    int
	NumUniqueLabelsPerPod         int
	NumUniqueLabelsPerDeployment  int
	NumSharedLabelsPerPod         int
	KubeconfigPath                string
	RestartNpmPods                bool
	DebugExitAfterPrintCounts     bool
	DebugExitAfterGeneration      bool
	SleepAfterCreation            time.Duration
	DeleteKwokPods                bool
	DeleteRealPods                bool
	DeletePodsInterval            time.Duration
	DeletePodsTimes               int
	DeleteLabels                  bool
	DeleteLabelsInterval          time.Duration
	DeleteLabelsTimes             int
	DeleteNetworkPolicies         bool
	DeleteNetworkPoliciesInterval time.Duration
	DeleteNetworkPoliciesTimes    int
	numKwokPods                   int
	numRealPods                   int
	LabelsToGetMetrics            map[string]string
	AdditionalTelemetryProperty   map[string]string
	CleanUp                       bool
}
