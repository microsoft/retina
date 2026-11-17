// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/labelsfilter"
	"github.com/cilium/cilium/pkg/option"
)

const (
	ciliumConfigNamespace = "kube-system"
	ciliumConfigName      = "cilium-config"

	// cilium-config keys, see github.com/cilium/cilium/pkg/option.
	ciliumConfigIdentityManagementMode = option.IdentityManagementMode
	ciliumConfigClusterName            = "cluster-name"
	ciliumConfigLabels                 = option.Labels
	ciliumConfigLabelPrefixFile        = option.LabelPrefixFile
)

var (
	ErrIdentityModeNotOperator = errors.New("--" + ReuseOperatorIdentities + " requires " +
		ciliumConfigIdentityManagementMode + "=" + option.IdentityManagementModeOperator + " in " + ciliumConfigNamespace + "/" + ciliumConfigName)
	ErrLabelPrefixFileUnsupported = errors.New(ciliumConfigLabelPrefixFile + " in " + ciliumConfigNamespace + "/" + ciliumConfigName +
		" is not supported with --" + ReuseOperatorIdentities + ": identity labels would not match cilium-operator's")
	ErrInvalidLabelPrefixCfg = errors.New("invalid " + ciliumConfigLabels + " in " + ciliumConfigNamespace + "/" + ciliumConfigName)
)

// setupOperatorIdentities makes identity labels match the CiliumIdentities created by cilium-operator.
// It configures the process-global label filter and cluster name from cilium-config, so it must run before any worker.
func (r *endpointReconciler) setupOperatorIdentities(ctx context.Context) error {
	if r.k8sClient == nil {
		return errors.Wrap(ErrClientsetDisabled, "kubernetes client is required to read "+ciliumConfigName)
	}

	getCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	cm, err := r.k8sClient.CoreV1().ConfigMaps(ciliumConfigNamespace).Get(getCtx, ciliumConfigName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to read "+ciliumConfigNamespace+"/"+ciliumConfigName)
	}

	if mode := cm.Data[ciliumConfigIdentityManagementMode]; mode != option.IdentityManagementModeOperator {
		return errors.Wrapf(ErrIdentityModeNotOperator, "got %q", mode)
	}

	if file := cm.Data[ciliumConfigLabelPrefixFile]; file != "" {
		return errors.Wrapf(ErrLabelPrefixFileUnsupported, "got %q", file)
	}

	// cilium reads the labels option with viper.GetStringSlice, which splits a string value on whitespace.
	prefixes := strings.Fields(cm.Data[ciliumConfigLabels])
	if err := labelsfilter.ParseLabelPrefixCfg(r.l, prefixes, nil, ""); err != nil {
		return errors.Wrapf(ErrInvalidLabelPrefixCfg, "%q: %v", cm.Data[ciliumConfigLabels], err)
	}

	// GetPodMetadata adds option.Config.ClusterName as the io.cilium.k8s.policy.cluster label.
	// Like cilium, fall back to the default cluster name when cilium-config does not set one.
	clusterName := cm.Data[ciliumConfigClusterName]
	if clusterName == "" {
		clusterName = defaults.ClusterName
	}
	if option.Config.ClusterName != clusterName {
		r.l.Info("using cluster name from "+ciliumConfigName, "clusterName", clusterName, "previousClusterName", option.Config.ClusterName)
	}
	option.Config.ClusterName = clusterName

	r.l.Info("reusing cilium-operator identities", "clusterName", clusterName, "labels", prefixes)
	return nil
}
