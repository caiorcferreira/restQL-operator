/*


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
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RestQLSpec defines the desired state of RestQL
type RestQLSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Config is an YAML with RestQL parameters
	// +optional
	Config string `json:"config,omitempty"`

	// Tenant is the mappings scope to be used
	// +optional
	Tenant string `json:"tenant,omitempty"`

	// Deployment defines the RestQL application
	// +optional
	Deployment apps.DeploymentSpec `json:"deployment,omitempty"`
}

// RestQLStatus defines the observed state of RestQL
type RestQLStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	AppliedQueries map[string]QueryNamespaceName `json:"appliedQueries,omitempty"`

	AppliedTenants map[string]string `json:"appliedTenants,omitempty"`

	ConfigHash string `json:"configHash,omitempty"`
}

// +kubebuilder:object:root=true

// RestQL is the Schema for the restqls API
type RestQL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestQLSpec   `json:"spec,omitempty"`
	Status RestQLStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestQLList contains a list of RestQL
type RestQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestQL `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestQL{}, &RestQLList{})
}
