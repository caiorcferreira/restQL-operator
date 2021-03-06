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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossv1alpha1 "github.com/b2wdigital/restQL-operator/api/v1alpha1"
)

// TenantMappingReconciler reconciles a TenantMapping object
type TenantMappingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=oss.b2w.io,resources=tenantmappings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=oss.b2w.io,resources=tenantmappings/status,verbs=get;update;patch

func (r *TenantMappingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("query", req.NamespacedName)

	tenant := &ossv1alpha1.TenantMapping{}
	err := r.Get(ctx, req.NamespacedName, tenant)
	switch {
	case apierrors.IsNotFound(err):
		err := r.reconcileDeletedTenant(ctx, log, req.NamespacedName)
		return ctrl.Result{}, err
	case err != nil:
		log.Error(err, "unable to fetch TenantMapping object")
		return ctrl.Result{}, err
	}

	err = r.reconcileInsertedTenant(ctx, log, tenant)
	return ctrl.Result{}, err
}

func (r *TenantMappingReconciler) reconcileInsertedTenant(ctx context.Context, log logr.Logger, tenant *ossv1alpha1.TenantMapping) error {
	patchConfig := map[string]interface{}{
		"mappings": tenant.Spec.Mappings,
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
		if patchRestql.Status.AppliedTenants == nil {
			patchRestql.Status.AppliedTenants = make(map[string]string)
		}

		qn := types.NamespacedName{Name: tenant.GetName(), Namespace: tenant.GetNamespace()}
		patchRestql.Status.AppliedTenants[qn.String()] = tenant.Spec.Tenant
		if err = r.Patch(ctx, patchRestql, client.MergeFrom(&restql)); err != nil {
			log.Error(err, "failed to update RestQL")
			continue
		}

		if err = RestartRestQL(ctx, r, log, &restql); err != nil {
			log.Error(err, "failed to restart RestQL")
		}
	}

	return nil
}

func (r *TenantMappingReconciler) reconcileDeletedTenant(ctx context.Context, log logr.Logger, namespacedName types.NamespacedName) error {
	instances := &ossv1alpha1.RestQLList{}
	if err := r.List(ctx, instances); err != nil {
		log.Error(err, "unable to list RestQL instances")
		return client.IgnoreNotFound(err)
	}

	for _, restql := range instances.Items {
		if restql.Status.AppliedTenants == nil {
			continue
		}

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

			delete(cfg, "mappings")

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

		patchRestql := restql.DeepCopy()
		delete(patchRestql.Status.AppliedTenants, namespacedName.String())
		if err = r.Patch(ctx, patchRestql, client.MergeFrom(&restql)); err != nil {
			log.Error(err, "failed to patch RestQL")
			continue
		}

		if err = RestartRestQL(ctx, r, log, &restql); err != nil {
			log.Error(err, "failed to restart RestQL")
		}
	}

	return nil
}

func (r *TenantMappingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ossv1alpha1.TenantMapping{}).
		Complete(r)
}
