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

func getCaptureAndPrintCaptureResult(ctx context.Context, kubeClient *kubernetes.Clientset, name, namespace string) error {
	return listCapturesAndPrintCaptureResults(ctx, kubeClient, name, namespace)
}

func listCapturesInNamespaceAndPrintCaptureResults(ctx context.Context, kubeClient *kubernetes.Clientset, namespace string) error {
	return listCapturesAndPrintCaptureResults(ctx, kubeClient, "", namespace)
}

// listCapturesAndPrintCaptureResults list captures and print the running jobs into properly aligned text.
func listCapturesAndPrintCaptureResults(ctx context.Context, kubeClient *kubernetes.Clientset, name, namespace string) error {
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
	fmt.Fprintln(w, "NAMESPACE\tCAPTURE NAME\tJOBS\tCOMPLETIONS\tAGE")
	for captureRef, jobs := range captureToJobs {
		captureRef := strings.Split(captureRef, "/")
		captureNamespace, captureName := captureRef[0], captureRef[1]
		jobNames := []string{}
		completedJobNum := 0
		age := ""
		totalJobNum := len(jobs)
		for _, job := range jobs {
			jobNames = append(jobNames, job.Name)
			if job.Status.CompletionTime != nil {
				completedJobNum += 1
			}
		}
		sort.SliceStable(jobNames, func(i, j int) bool {
			return jobNames[i] < jobNames[j]
		})
		if len(jobs) > 0 {
			age = durationUtil.HumanDuration(time.Since(jobs[0].CreationTimestamp.Time))
		}

		jobsNameJoined := strings.Join(jobNames, ",")

		completions := fmt.Sprintf("%d/%d", completedJobNum, totalJobNum)
		rr := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t", captureNamespace, captureName, jobsNameJoined, completions, age)
		fmt.Fprintln(w, rr)
	}
	w.Flush()
	fmt.Println()
}
