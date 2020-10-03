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
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

	err := r.reconcileConfig(ctx, log, restql)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("config reconciled")

	err = r.reconcileDeploy(ctx, log, restql)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("deploy reconciled")

	return ctrl.Result{}, nil
}

func (r *RestQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ossv1alpha1.RestQL{}).
		Owns(&apps.Deployment{}).WithEventFilter(&predicate.GenerationChangedPredicate{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *RestQLReconciler) reconcileConfig(ctx context.Context, log logr.Logger, restql *ossv1alpha1.RestQL) error {
	newConfigMap, err := r.newConfigMap(restql)
	if err != nil {
		return err
	}

	currentConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      newConfigMap.GetName(),
		Namespace: newConfigMap.GetNamespace(),
	}, currentConfigMap)
	switch {
	case apierrors.IsNotFound(err):
		err := r.Create(ctx, newConfigMap)
		if err != nil {
			log.Error(err, "unexpected error when creating config map")
			return client.IgnoreNotFound(err)
		}
	case err != nil:
		log.Error(err, "unexpected error when fetching config map")
		return err
	}

	err = r.Update(ctx, newConfigMap)
	if err != nil {
		log.Error(err, "unexpected error when updating config map")
		return client.IgnoreNotFound(err)
	}

	return nil
}

func (r *RestQLReconciler) reconcileDeploy(ctx context.Context, log logr.Logger, restql *ossv1alpha1.RestQL) error {
	if restql.Spec.Deployment.String() == "nil" {
		return nil
	}

	newDeployment, err := r.newDeployment(restql)
	if err != nil {
		return err
	}

	currentDeployment := &apps.Deployment{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      newDeployment.GetName(),
		Namespace: newDeployment.GetNamespace(),
	}, currentDeployment)
	switch {
	case apierrors.IsNotFound(err):
		err := r.Create(ctx, newDeployment)
		if err != nil {
			log.Error(err, "unexpected error when creating deployment")
			return client.IgnoreNotFound(err)
		}
	case err != nil:
		log.Error(err, "unexpected error when fetching deployment")
		return err
	}

	err = r.Update(ctx, newDeployment)
	if err != nil {
		log.Error(err, "unexpected error when updating deployment")
		return client.IgnoreNotFound(err)
	}

	return nil
}

func (r *RestQLReconciler) newConfigMap(cr *ossv1alpha1.RestQL) (*corev1.ConfigMap, error) {
	labels := map[string]string{
		"app": cr.Name,
	}
	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"restql.yml": cr.Spec.Config,
		},
	}

	err := ctrl.SetControllerReference(cr, cfg, r.Scheme)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (r *RestQLReconciler) newDeployment(cr *ossv1alpha1.RestQL) (*apps.Deployment, error) {
	labels := map[string]string{
		"app": cr.Name,
	}

	configFileLocation := "/restql/config"
	configFileName := "restql.yml"

	templateSpec := cr.Spec.Deployment.Template.Spec

	templateSpec.Volumes = append(templateSpec.Volumes, corev1.Volume{
		Name: "restql-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName(cr),
				},
				Items: []corev1.KeyToPath{
					{Key: configFileName, Path: configFileName},
				},
			},
		},
	})
	for i, container := range templateSpec.Containers {
		vm := corev1.VolumeMount{
			Name:      "restql-config",
			MountPath: configFileLocation,
		}

		cfgVar := corev1.EnvVar{
			Name:  "RESTQL_CONFIG",
			Value: path.Join(configFileLocation, configFileName),
		}

		templateSpec.Containers[i].VolumeMounts = append(container.VolumeMounts, vm)
		templateSpec.Containers[i].Env = append(container.Env, cfgVar)
	}

	cr.Spec.Deployment.Template.Spec = templateSpec
	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-deployment",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: cr.Spec.Deployment,
	}

	err := ctrl.SetControllerReference(cr, d, r.Scheme)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func configMapName(cr *ossv1alpha1.RestQL) string {
	return cr.Name + "-config"
}
