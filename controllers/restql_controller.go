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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossv1alpha1 "github.com/b2wdigital/restQL-operator/api/v1alpha1"
)

// RestQLReconciler reconciles a RestQL object
type RestQLReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=oss.b2w.io,resources=restqls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=oss.b2w.io,resources=restqls/status,verbs=get;update;patch

func (r *RestQLReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("restql", req.NamespacedName)

	restql := &ossv1alpha1.RestQL{}
	if err := r.Get(ctx, req.NamespacedName, restql); err != nil {
		log.Error(err, "unable to fetch restQL object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	newConfigMap := newConfigMap(restql)
	currentConfigMap := &corev1.ConfigMap{}

	err := r.Get(ctx, client.ObjectKey{Name: newConfigMap.GetName(), Namespace: newConfigMap.GetNamespace()}, currentConfigMap)
	switch {
	case apierrors.IsNotFound(err):
		err := r.Create(ctx, newConfigMap)
		if err != nil {
			log.Error(err, "unexpected error when creating config map")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	case err != nil:
		log.Error(err, "unexpected error when fetching config map")
		return ctrl.Result{}, err
	}

	err = r.Update(ctx, newConfigMap)
	if err != nil {
		log.Error(err, "unexpected error when updating config map")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

func (r *RestQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ossv1alpha1.RestQL{}).
		Complete(r)
}

func newConfigMap(cr *ossv1alpha1.RestQL) *corev1.ConfigMap {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-config",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"restql.yml": cr.Spec.Config,
		},
	}
}
