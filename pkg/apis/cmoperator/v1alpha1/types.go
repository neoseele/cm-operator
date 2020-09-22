/*
Copyright 2017 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomMetric is a specification for a CustomMetric resource
type CustomMetric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomMetricSpec   `json:"spec"`
	Status CustomMetricStatus `json:"status"`
}

// CustomMetricSpec is the spec for a CustomMetric resource
type CustomMetricSpec struct {
	Project  string   `json:"project"`
	Cluster  string   `json:"cluster"`
	Location string   `json:"location"`
	Metrics  []string `json:"metrics"`
}

// CustomMetricStatus is the status for a CustomMetric resource
type CustomMetricStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomMetricList is a list of CustomMetric resources
type CustomMetricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomMetric `json:"items"`
}
