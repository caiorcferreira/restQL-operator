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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossv1alpha1 "github.com/b2wdigital/restQL-operator/api/v1alpha1"
)

// QueryReconciler reconciles a Query object
type QueryReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=oss.b2w.io,resources=queries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=oss.b2w.io,resources=queries/status,verbs=get;update;patch

func (r *QueryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("query", req.NamespacedName)

	query := &ossv1alpha1.Query{}
	if err := r.Get(ctx, req.NamespacedName, query); err != nil {
		log.Error(err, "unable to fetch Query object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchConfig := map[string]interface{}{
		"queries": map[string]interface{}{
			query.Spec.Namespace: map[string]interface{}{
				query.Spec.Name: query.Spec.Revisions,
			},
		},
	}

	instances := &ossv1alpha1.RestQLList{}
	if err := r.List(ctx, instances); err != nil {
		log.Error(err, "unable to list RestQL instances")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, restql := range instances.Items {
		configList := &corev1.ConfigMapList{}
		err := r.List(ctx, configList, client.MatchingFields{configOwnerKey: restql.Name})
		if err != nil {
			log.Error(err, "failed to fetch config maps")
			continue
		}

		log.V(1).Info("fetched config maps", "config", configList.String())

		for _, c := range configList.Items {
			yamlCfg := c.Data["restql.yml"]
			mergedYaml, err := mergeYamlConfig(yamlCfg, patchConfig)
			if err != nil {
				log.Error(err, "failed to merge YAML")
				continue
			}

			log.V(1).Info("merged configuration successfully", "config", mergedYaml)

			c.Data["restql.yml"] = mergedYaml
			if err = r.Update(ctx, &c); err != nil {
				log.Error(err, "failed to update config maps")
			}
		}

	}

	return ctrl.Result{}, nil
}

func mergeYamlConfig(yamlCfg string, patchConfig map[string]interface{}) (string, error) {
	cfg := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlCfg), &cfg)
	if err != nil {
		return "", err
	}

	err = mergo.Merge(&cfg, patchConfig)
	if err != nil {
		return "", err
	}

	mergedYamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}

	return string(mergedYamlBytes), nil
}

func (r *QueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ossv1alpha1.Query{}).
		Complete(r)
}
