/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// +kubebuilder:docs-gen:collapse=Apache License

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/internal/appmgrcli/disruption"
	openapiruntime "github.com/go-openapi/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHookStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, "200"},
		{"424 response", disruption.NewPrepareApplicationFailedDependency(), "424"},
		{"425 response", disruption.NewCheckReadinessStatus425(), "425"},
		{"accept 424 response", disruption.NewAcceptDisruptionFailedDependency(), "424"},
		{"other response", disruption.NewPrepareApplicationDefault(503), "503"},
		{"unexpected success response", openapiruntime.NewAPIError("unexpected success response", nil, 204), "204"},
		{"wrapped response", fmt.Errorf("hook: %w", disruption.NewPrepareApplicationFailedDependency()), "424"},
		{"no response", &url.Error{Op: "Post", URL: "https://app/prepare", Err: errors.New("connection refused")}, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hookStatusCode(tt.err))
		})
	}
}

func TestReadyHookMetricRecordsResponseCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusFailedDependency)
	}))
	defer server.Close()

	resolver := &ApplicationDisruptionBudgetResolver{
		ApplicationDisruptionBudget: &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{Name: "metric-test", Namespace: "metric-test"},
			Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
				HookV2BasePath: nodedisruptionv1alpha1.HookSpec{URL: server.URL + "/api/v2"},
			},
		},
	}
	adb := resolver.ApplicationDisruptionBudget
	counter424 := DisruptionBudgetCheckReadyHookStatusCodeTotal.WithLabelValues(adb.Namespace, adb.Name, adb.Kind, "424")
	counter200 := DisruptionBudgetCheckReadyHookStatusCodeTotal.WithLabelValues(adb.Namespace, adb.Name, adb.Kind, "200")
	before424, before200 := counterValue(t, counter424), counterValue(t, counter200)

	err := resolver.CallReadyHook(context.Background(), nodedisruptionv1alpha1.NodeDisruption{}, time.Second)

	require.Error(t, err)
	assert.Equal(t, 1.0, counterValue(t, counter424)-before424, "a 424 answer must be counted as 424")
	assert.Equal(t, 0.0, counterValue(t, counter200)-before200, "a 424 answer must not be counted as 200")
}
