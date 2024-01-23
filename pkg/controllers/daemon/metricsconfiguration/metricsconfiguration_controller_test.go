/*
Copyright 2023.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metricsconfigurationcontroller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
)

var fakescheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(fakescheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(fakescheme))

	_ = clientgoscheme.AddToScheme(fakescheme)
	fakescheme.AddKnownTypes(retinav1alpha1.GroupVersion, &retinav1alpha1.RetinaEndpoint{})
}

func TestMetricsConfigurationReconciler_Reconcile(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	type fields struct {
		existingObjects []client.Object
	}
	type args struct {
		ctx context.Context
		req ctrl.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
		wantNil bool
	}{
		{
			name: "should create a new metrics configuration",
			fields: fields{
				existingObjects: []client.Object{
					&retinav1alpha1.MetricsConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				},
			},
			wantNil: false,
		},
		{
			name: "should send nil to channel when metrics configuration is deleted",
			fields: fields{
				existingObjects: []client.Object{},
			},
			args: args{
				ctx: context.TODO(),
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				},
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(tt.fields.existingObjects...).Build()

			r := New(client, fakescheme, nil)

			_, err := r.Reconcile(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				require.Error(t, err)
			}

			// reactivate when this channel is used again
			// verify that the pod is deleted
			/*
				select {
				case config := <-metricsconfigchannel:
					if tt.wantNil {
						require.Nil(t, config.MetricsConfiguration)
					} else {
						require.NotNil(t, config.MetricsConfiguration)
					}

				case <-time.After(5 * time.Second):
					require.Errorf(t, nil, "controller didn't write to channel after allotted time")
				}
			*/
		})
	}
}
