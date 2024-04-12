// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var allNamespaces bool

var listExample = templates.Examples(i18n.T(`
		# List Retina Capture in namespace "capture"
		kubectl retina capture list -n capture

		# List Retina Capture in all namespaces
		kubectl retina capture list --all-namespaces
	`))

var list = &cobra.Command{
	Use:     "list",
	Short:   "List Retina Captures",
	Example: listExample,
	RunE: func(*cobra.Command, []string) error {
		kubeConfig, err := configFlags.ToRESTConfig()
		if err != nil {
			return err
		}

		kubeClient, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return err
		}

		captureNamespace := namespace
		if allNamespaces {
			captureNamespace = ""
		}
		return listCapturesInNamespaceAndPrintCaptureResults(kubeClient, captureNamespace)
	},
}

func init() {
	capture.AddCommand(list)
	list.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to host capture job")
	list.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", allNamespaces,
		"If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
}
