// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	retinacmd "github.com/microsoft/retina/cli/cmd"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var deleteExample = templates.Examples(i18n.T(`
		# Delete the Retina Capture "retina-capture-8v6wd" in namespace "capture"
		kubectl retina capture delete --name retina-capture-8v6wd --namespace capture
		`))

var deleteCapture = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a Retina capture",
	Example: deleteExample,
	RunE: func(*cobra.Command, []string) error {
		kubeConfig, err := opts.ToRESTConfig()
		if err != nil {
			return errors.Wrap(err, "")
		}

		kubeClient, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return errors.Wrap(err, "")
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
		defer cancel()

		captureJobSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				label.CaptureNameLabel: *opts.Name,
				label.AppLabel:         captureConstants.CaptureAppname,
			},
		}
		labelSelector, _ := labels.Parse(metav1.FormatLabelSelector(captureJobSelector))
		jobListOpt := metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		}

		jobList, err := kubeClient.BatchV1().Jobs(*opts.Namespace).List(ctx, jobListOpt)
		if err != nil {
			return errors.Wrap(err, "failed to list capture jobs")
		}
		if len(jobList.Items) == 0 {
			return errors.Errorf("capture %s in namespace %s was not found", *opts.Name, *opts.Namespace)
		}

		for _, job := range jobList.Items {
			deletePropagationBackground := metav1.DeletePropagationBackground
			if err := kubeClient.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{
				PropagationPolicy: &deletePropagationBackground,
			}); err != nil {
				retinacmd.Logger.Info("Failed to delete job", zap.String("job name", job.Name), zap.Error(err))
			}
		}

		for _, volume := range jobList.Items[0].Spec.Template.Spec.Volumes {
			if volume.Secret != nil {
				if err := kubeClient.CoreV1().Secrets(*opts.Namespace).Delete(ctx, volume.Secret.SecretName, metav1.DeleteOptions{}); err != nil {
					return errors.Wrap(err, "failed to delete capture secret")
				}
				break
			}
		}
		retinacmd.Logger.Info(fmt.Sprintf("Retina Capture %q delete", *opts.Name))

		return nil
	},
}

func init() {
	capture.AddCommand(deleteCapture)
}
