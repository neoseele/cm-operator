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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cmv1 "github.com/neoseele/cm-operator/api/v1"
)

// CustomMetricReconciler reconciles a CustomMetric object
type CustomMetricReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func int32Ptr(i int32) *int32 { return &i }

// +kubebuilder:rbac:groups=cm.example.com,resources=custommetrics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cm.example.com,resources=custommetrics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cm.example.com,resources=custommetrics/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac,resources=clustlerroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac,resources=clustlerrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;

// Reconcile entry
func (r *CustomMetricReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("custommetric", req.NamespacedName)

	// Fetch the CustomMetric instance
	instance := &cmv1.CustomMetric{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("CustomMetric resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get CustomMetric")
		return ctrl.Result{}, err
	}

	// ClusterRole
	if err := r.reconcileClusterRole(ctx, instance, log); err != nil {
		return ctrl.Result{}, err
	}

	// ClusterRoleBinding
	if err := r.reconcileClusterRoleBinding(ctx, instance, log); err != nil {
		return ctrl.Result{}, err
	}

	// ConfigMap
	if err := r.reconcileConfigMap(ctx, instance, log); err != nil {
		return ctrl.Result{}, err
	}

	// Deployment
	if err := r.reconcileDeployment(ctx, instance, log); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager specifies how the controller is built to watch a CR and other resources that are owned and managed by that controller
func (r *CustomMetricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cmv1.CustomMetric{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

// reconcile functions

func (r *CustomMetricReconciler) reconcileClusterRole(ctx context.Context, cr *cmv1.CustomMetric, reqLogger logr.Logger) error {
	// Define a new object
	clusterRole := newClusterRole(cr)

	// Set CustomMetric instance as the owner and controller
	// if err := controllerutil.SetControllerReference(cr, clusterRole, r.scheme); err != nil {
	//   return err
	// }

	// Check if this object already exists
	found := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new clusterRole name", "clusterRole.Name", clusterRole.Name)
		err = r.Create(ctx, clusterRole)
		if err != nil {
			return err
		}

		// object created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// object already exists - don't requeue
	reqLogger.Info("Skip reconcile: ConfigMap already exists", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
	return nil
}

func (r *CustomMetricReconciler) reconcileClusterRoleBinding(ctx context.Context, cr *cmv1.CustomMetric, reqLogger logr.Logger) error {
	// Define a new object
	clusterRoleBinding := newClusterRoleBinding(cr)

	// Set CustomMetric instance as the owner and controller
	// if err := controllerutil.SetControllerReference(cr, clusterRoleBinding, r.scheme); err != nil {
	//   return err
	// }

	// Check if this object already exists
	found := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new clusterRoleBinding", "clusterRoleBinding.Name", clusterRoleBinding.Name)
		err = r.Create(ctx, clusterRoleBinding)
		if err != nil {
			return err
		}

		// object created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// object already exists - don't requeue
	reqLogger.Info("Skip reconcile: ConfigMap already exists", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
	return nil
}

func (r *CustomMetricReconciler) reconcileConfigMap(ctx context.Context, cr *cmv1.CustomMetric, reqLogger logr.Logger) error {
	// Define a new object
	configmap := newConfigMap(cr)

	// Set CustomMetric instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, configmap, r.Scheme); err != nil {
		return err
	}

	// Check if this object already exists
	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configmap.Name, Namespace: configmap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configmap.Namespace, "ConfigMap.Name", configmap.Name)
		err = r.Create(ctx, configmap)
		if err != nil {
			return err
		}

		// object created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// object already exists - don't requeue
	reqLogger.Info("Skip reconcile: ConfigMap already exists", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
	return nil
}

func (r *CustomMetricReconciler) reconcileDeployment(ctx context.Context, cr *cmv1.CustomMetric, reqLogger logr.Logger) error {
	// Define a new object
	deployment := newDeployment(cr)

	// Set CustomMetric instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, deployment, r.Scheme); err != nil {
		return err
	}

	// Check if this object already exists
	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.Create(ctx, deployment)
		if err != nil {
			return err
		}

		// object created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// object already exists - don't requeue
	reqLogger.Info("Skip reconcile: Deployment already exists", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	return nil
}

func newClusterRole(cr *cmv1.CustomMetric) *rbacv1.ClusterRole {
	resourceName := cr.Name + "-prometheus"

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, cmv1.SchemeBuilder.GroupVersion.WithKind("CustomMetric")),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"nodes",
					"nodes/proxy",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"extentions",
				},
				Resources: []string{
					"ingresses",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				NonResourceURLs: []string{
					"/metrics",
				},
				Verbs: []string{
					"get",
				},
			},
		},
	}
}

func newClusterRoleBinding(cr *cmv1.CustomMetric) *rbacv1.ClusterRoleBinding {
	resourceName := cr.Name + "-prometheus"

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, cmv1.SchemeBuilder.GroupVersion.WithKind("CustomMetric")),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: cr.Namespace,
			},
		},
	}
}

func newConfigMap(cr *cmv1.CustomMetric) *corev1.ConfigMap {
	resourceName := cr.Name + "-prometheus"

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: cr.Namespace,
			// OwnerReferences: []metav1.OwnerReference{
			//   *metav1.NewControllerRef(cr, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
			// },
		},
		Data: map[string]string{
			"prometheus.yml": `
scrape_configs:

- job_name: 'kubernetes-pods'

  kubernetes_sd_configs:
    - role: pod

  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_cm_example_com_scrape]
      action: keep
      regex: true
    - source_labels:
      - __meta_kubernetes_pod_annotationpresent_cm_example_com_path
      - __meta_kubernetes_pod_annotation_cm_example_com_path
      action: replace
      target_label: __metrics_path__
      regex: true;(.+)
    - source_labels: [__address__, __meta_kubernetes_pod_annotation_cm_example_com_port]
      action: replace
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_pod_label_(.+)
    - source_labels: [__meta_kubernetes_namespace]
      action: replace
      target_label: kubernetes_namespace
    - source_labels: [__meta_kubernetes_pod_name]
      action: replace
      target_label: kubernetes_pod_name

- job_name: 'kubernetes-nodes-cadvisor'

  metrics_path: /metrics/cadvisor

  kubernetes_sd_configs:
    - role: node

  relabel_configs:
    - source_labels: [__meta_kubernetes_node_annotation_cm_example_com_scrape]
      action: keep
      regex: true
    - source_labels: [__address__]
      action: replace
      regex: ([^:]+)(?::\d+)?
      replacement: $1:10255
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_node_label_(.+)
      `,
		},
	}
}

func newDeployment(cr *cmv1.CustomMetric) *appsv1.Deployment {
	resourceName := cr.Name + "-prometheus"

	labels := map[string]string{
		"app":        "prometheus-server",
		"controller": cr.Name,
	}

	sidecarArgs := []string{
		fmt.Sprintf("--stackdriver.project-id=%s", cr.Spec.Project),
		fmt.Sprintf("--stackdriver.kubernetes.cluster-name=%s", cr.Spec.Cluster),
		fmt.Sprintf("--stackdriver.kubernetes.location=%s", cr.Spec.Location),
		"--prometheus.wal-directory=/prometheus/wal",
		"--log.level=debug",
	}

	for _, m := range cr.Spec.Metrics {
		sidecarArgs = append(sidecarArgs, fmt.Sprintf("--include={__name__=~\"%s\"}", m))
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: cr.Namespace,
			// OwnerReferences: []metav1.OwnerReference{
			//   *metav1.NewControllerRef(cr, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
			// },
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "prometheus",
							Image: "prom/prometheus:v2.6.1",
							Args: []string{
								"--config.file=/etc/prometheus/prometheus.yml",
								"--storage.tsdb.path=/prometheus/",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9090,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "prometheus-config-volume",
									MountPath: "/etc/prometheus/",
								},
								{
									Name:      "prometheus-storage-volume",
									MountPath: "/prometheus/",
								},
							},
						},
						{
							Name:  "sidecar",
							Image: "gcr.io/stackdriver-prometheus/stackdriver-prometheus-sidecar:0.8.0",
							Args:  sidecarArgs,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9091,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "prometheus-storage-volume",
									MountPath: "/prometheus/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "prometheus-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: resourceName,
									},
									DefaultMode: int32Ptr(420),
								},
							},
						},
						{
							Name: "prometheus-storage-volume", // default to emptyDir
						},
					},
				},
			},
		},
	}
}
