// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	durationUtil "k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
)

func getCaptureAndPrintCaptureResult(ctx context.Context, kubeClient kubernetes.Interface, name, namespace string) error {
	return listCapturesAndPrintCaptureResults(ctx, kubeClient, name, namespace)
}

func listCapturesInNamespaceAndPrintCaptureResults(ctx context.Context, kubeClient *kubernetes.Clientset, namespace string) error {
	return listCapturesAndPrintCaptureResults(ctx, kubeClient, "", namespace)
}

// listCapturesAndPrintCaptureResults list captures and print the running jobs into properly aligned text.
func listCapturesAndPrintCaptureResults(ctx context.Context, kubeClient kubernetes.Interface, name, namespace string) error {
	jobListOpt := metav1.ListOptions{}
	if len(name) != 0 {
		captureJobSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				label.CaptureNameLabel: name,
				label.AppLabel:         captureConstants.CaptureAppname,
			},
		}
		labelSelector, _ := labels.Parse(metav1.FormatLabelSelector(captureJobSelector))
		jobListOpt = metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		}
	}

	jobList, err := kubeClient.BatchV1().Jobs(namespace).List(ctx, jobListOpt)
	if err != nil {
		return err
	}
	if len(jobList.Items) == 0 {
		fmt.Printf("No Capture found in %s namespace.\n", namespace)
		return nil
	}
	printCaptureResult(jobList.Items)
	return nil
}

func printCaptureResult(captureJobs []batchv1.Job) {
	if len(captureJobs) == 0 {
		return
	}
	captureToJobs := map[string][]batchv1.Job{}

	for _, job := range captureJobs {
		captureName, ok := job.Labels[label.CaptureNameLabel]
		if !ok {
			continue
		}
		captureRef := fmt.Sprintf("%s/%s", job.Namespace, captureName)
		captureToJobs[captureRef] = append(captureToJobs[captureRef], job)
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tCAPTURE NAME\tJOB\tCOMPLETIONS\tAGE")

	for captureRef := range captureToJobs {
		jobs := captureToJobs[captureRef]
		captureParts := strings.Split(captureRef, "/")
		captureNamespace, captureName := captureParts[0], captureParts[1]

		sort.SliceStable(jobs, func(i, j int) bool {
			return jobs[i].Name < jobs[j].Name
		})

		for i := range jobs {
			job := &jobs[i]
			var completions string
			if job.Spec.Completions != nil {
				completions = fmt.Sprintf("%d/%d", job.Status.Succeeded, *job.Spec.Completions)
			}
			age := durationUtil.HumanDuration(time.Since(job.CreationTimestamp.Time))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", captureNamespace, captureName, job.Name, completions, age)
		}
	}
	w.Flush()
	fmt.Println()
}
