// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
)

func TestGetPodAnnotationsFromCapture(t *testing.T) {
	startTime := &metav1.Time{Time: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)}
	rawHostPath := "my-capture"
	resolvedHostPath := "/var/log/retina/captures/my-capture"

	captureWithHostPath := func() *retinav1alpha1.Capture {
		hp := rawHostPath
		return &retinav1alpha1.Capture{
			ObjectMeta: metav1.ObjectMeta{Name: "cap1"},
			Spec: retinav1alpha1.CaptureSpec{
				OutputConfiguration: retinav1alpha1.OutputConfiguration{HostPath: &hp},
			},
			Status: retinav1alpha1.CaptureStatus{StartTime: startTime},
		}
	}

	cases := []struct {
		name             string
		capture          *retinav1alpha1.Capture
		resolvedHostPath string
		wantHostPathAnn  string // empty means annotation must be absent
	}{
		{
			// This is the case that caused the download bug: capture is created
			// with a relative subpath, the operator resolves it to an absolute
			// on-node path, and the annotation must carry the resolved value
			// (because `kubectl retina capture download` mounts it verbatim).
			name:             "resolved path wins over raw spec value",
			capture:          captureWithHostPath(),
			resolvedHostPath: resolvedHostPath,
			wantHostPathAnn:  resolvedHostPath,
		},
		{
			// Fallback path for callers that don't have a resolved value (e.g.
			// CLI helpers that only see the CR). The raw spec value is used.
			name:             "raw spec value used when resolved is empty",
			capture:          captureWithHostPath(),
			resolvedHostPath: "",
			wantHostPathAnn:  rawHostPath,
		},
		{
			// Defense-in-depth: even if the spec has no HostPath, an explicit
			// resolved value should still be written (callers own the choice).
			name: "resolved path is written even when spec.HostPath is nil",
			capture: &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{Name: "cap2"},
				Status:     retinav1alpha1.CaptureStatus{StartTime: startTime},
			},
			resolvedHostPath: resolvedHostPath,
			wantHostPathAnn:  resolvedHostPath,
		},
		{
			name: "no HostPath annotation when neither resolved nor spec is set",
			capture: &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{Name: "cap3"},
				Status:     retinav1alpha1.CaptureStatus{StartTime: startTime},
			},
			resolvedHostPath: "",
			wantHostPathAnn:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GetPodAnnotationsFromCapture(tc.capture, tc.resolvedHostPath)

			// Filename annotation is always set to the capture name.
			assert.Equal(t, tc.capture.Name, got[captureConstants.CaptureFilenameAnnotationKey])

			if tc.wantHostPathAnn == "" {
				_, ok := got[captureConstants.CaptureHostPathAnnotationKey]
				assert.False(t, ok, "HostPath annotation should be absent")
			} else {
				assert.Equal(t, tc.wantHostPathAnn, got[captureConstants.CaptureHostPathAnnotationKey])
			}
		})
	}
}

func TestGetPodAnnotationsFromCapture_TimestampOnlyWhenStartTimeSet(t *testing.T) {
	hp := "my-capture"
	base := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{Name: "cap"},
		Spec: retinav1alpha1.CaptureSpec{
			OutputConfiguration: retinav1alpha1.OutputConfiguration{HostPath: &hp},
		},
	}

	t.Run("no StartTime -> no timestamp annotation", func(t *testing.T) {
		got := GetPodAnnotationsFromCapture(base, "")
		_, ok := got[captureConstants.CaptureTimestampAnnotationKey]
		assert.False(t, ok)
	})

	t.Run("StartTime set -> timestamp annotation populated", func(t *testing.T) {
		ts := &metav1.Time{Time: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)}
		c := base.DeepCopy()
		c.Status.StartTime = ts

		got := GetPodAnnotationsFromCapture(c, "")
		require.Equal(t, file.TimeToString(ts), got[captureConstants.CaptureTimestampAnnotationKey])
	})
}
