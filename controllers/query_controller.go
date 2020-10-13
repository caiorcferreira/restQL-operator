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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	err := r.Get(ctx, req.NamespacedName, query)
	switch {
	case apierrors.IsNotFound(err):
		err := r.reconcileDeletedQuery(ctx, log, req.NamespacedName)
		return ctrl.Result{}, err
	case err != nil:
		log.Error(err, "unable to fetch Query object")
		return ctrl.Result{}, err
	}

	err = r.reconcileInsertedQuery(ctx, log, query)
	return ctrl.Result{}, err
}

func (r *QueryReconciler) reconcileInsertedQuery(ctx context.Context, log logr.Logger, query *ossv1alpha1.Query) error {
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
		return client.IgnoreNotFound(err)
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
			yamlCfg := c.Data[restQLConfigFilename]
			mergedYaml, err := mergeYamlConfig(yamlCfg, patchConfig)
			if err != nil {
				log.Error(err, "failed to merge YAML")
				continue
			}

			log.V(1).Info("merged configuration successfully", "config", mergedYaml)

			patch := c.DeepCopy()
			patch.Data[restQLConfigFilename] = mergedYaml
			if err = r.Patch(ctx, patch, client.MergeFrom(&c)); err != nil {
				log.Error(err, "failed to patch config maps")
			}
		}

		patchRestql := restql.DeepCopy()
		if patchRestql.Status.AppliedQueries == nil {
			patchRestql.Status.AppliedQueries = make(map[string]ossv1alpha1.QueryNamespaceName)
		}

		qn := types.NamespacedName{Name: query.GetName(), Namespace: query.GetNamespace()}
		patchRestql.Status.AppliedQueries[qn.String()] = ossv1alpha1.QueryNamespaceName{Namespace: query.Spec.Namespace, Name: query.Spec.Name}
		if err = r.Patch(ctx, patchRestql, client.MergeFrom(&restql)); err != nil {
			log.Error(err, "failed to patch RestQL")
		}
	}

	return nil
}

func (r *QueryReconciler) reconcileDeletedQuery(ctx context.Context, log logr.Logger, namespacedName types.NamespacedName) error {
	instances := &ossv1alpha1.RestQLList{}
	if err := r.List(ctx, instances); err != nil {
		log.Error(err, "unable to list RestQL instances")
		return client.IgnoreNotFound(err)
	}

	for _, restql := range instances.Items {
		if restql.Status.AppliedQueries == nil {
			continue
		}

		queryNamespaceName := restql.Status.AppliedQueries[namespacedName.String()]

		configList := &corev1.ConfigMapList{}
		err := r.List(ctx, configList, client.MatchingFields{configOwnerKey: restql.Name})
		if err != nil {
			log.Error(err, "failed to fetch config maps")
			continue
		}

		log.V(1).Info("fetched config maps", "config", configList.String())

		for _, c := range configList.Items {
			yamlCfg := c.Data[restQLConfigFilename]
			cfg := make(map[string]interface{})
			if err := yaml.Unmarshal([]byte(yamlCfg), cfg); err != nil {
				log.Error(err, "failed to unmarshal config")
				continue
			}

			queries, ok := cfg["queries"].(map[interface{}]interface{})
			if !ok {
				log.Error(err, "queries field does not exist or is not a map")
				continue
			}

			namespace, ok := queries[queryNamespaceName.Namespace].(map[interface{}]interface{})
			if !ok {
				log.Error(err, "namespace field does not exist or is not a map")
				continue
			}

			delete(namespace, queryNamespaceName.Name)

			bytes, err := yaml.Marshal(cfg)
			if err != nil {
				log.Error(err, "failed to marshal config")
				continue
			}

			updatedYaml := string(bytes)

			patch := c.DeepCopy()
			patch.Data[restQLConfigFilename] = updatedYaml
			if err = r.Patch(ctx, patch, client.MergeFrom(&c)); err != nil {
				log.Error(err, "failed to patch config maps")
			}

			log.V(1).Info("deleted query from configuration successfully", "config", updatedYaml)
		}

		restqlPatch := restql.DeepCopy()
		delete(restqlPatch.Status.AppliedQueries, namespacedName.String())
		if err = r.Patch(ctx, restqlPatch, client.MergeFrom(&restql)); err != nil {
			log.Error(err, "failed to patch RestQL")
		}
	}

	return nil
}

func (r *QueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ossv1alpha1.Query{}).
		Complete(r)
}
