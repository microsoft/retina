// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
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

func NewListSubCommand() *cobra.Command {
	listCaptures := &cobra.Command{
		Use:     "list",
		Short:   "List Retina Captures",
		Example: listExample,
		RunE: func(*cobra.Command, []string) error {
			kubeConfig, err := opts.ToRESTConfig()
			if err != nil {
				return errors.Wrap(err, "failed to compose k8s rest config")
			}

			kubeClient, err := kubernetes.NewForConfig(kubeConfig)
			if err != nil {
				return errors.Wrap(err, "failed to initialize kubernetes client")
			}

			// Create a context that is canceled when a termination signal is received
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
			defer cancel()

			captureNamespace := *opts.Namespace
			if allNamespaces {
				captureNamespace = ""
			}
			return listCapturesInNamespaceAndPrintCaptureResults(ctx, kubeClient, captureNamespace)
		},
	}

	listCaptures.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", allNamespaces,
		"If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	return listCaptures
}
