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

package main

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	cmv1alpha1 "k8s.io/cm-operator/pkg/apis/cmoperator/v1alpha1"
	clientset "k8s.io/cm-operator/pkg/generated/clientset/versioned"
	cmscheme "k8s.io/cm-operator/pkg/generated/clientset/versioned/scheme"
	informers "k8s.io/cm-operator/pkg/generated/informers/externalversions/cmoperator/v1alpha1"
	listers "k8s.io/cm-operator/pkg/generated/listers/cmoperator/v1alpha1"
)

const controllerAgentName = "cm-operator"

// const configmapName = "prometheus-server-conf"
// const clusterRoleName = "prometheus"
// const clusterRoleBindingName = "prometheus"

const (
	// SuccessSynced is used as part of the Event 'reason' when a CustomMetric is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a CustomMetric fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by CustomMetric"
	// MessageResourceSynced is the message used for an Event fired when a CustomMetric
	// is synced successfully
	MessageResourceSynced = "CustomMetric synced successfully"
)

// Controller is the controller implementation for CustomMetric resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// cmclientset is a clientset for our own API group
	cmclientset clientset.Interface

	clusterRolesLister        rbaclisters.ClusterRoleLister
	clusterRolesSynced        cache.InformerSynced
	clusterRoleBindingsLister rbaclisters.ClusterRoleBindingLister
	clusterRoleBindingsSynced cache.InformerSynced
	configmapsLister          corelisters.ConfigMapLister
	configmapsSynced          cache.InformerSynced
	deploymentsLister         appslisters.DeploymentLister
	deploymentsSynced         cache.InformerSynced
	customMetricsLister       listers.CustomMetricLister
	customMetricsSynced       cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new prom controller
func NewController(
	kubeclientset kubernetes.Interface,
	cmclientset clientset.Interface,
	clusterRoleInformer rbacinformers.ClusterRoleInformer,
	clusterRoleBindingInformer rbacinformers.ClusterRoleBindingInformer,
	configmapInformer coreinformers.ConfigMapInformer,
	deploymentInformer appsinformers.DeploymentInformer,
	customMetricInformer informers.CustomMetricInformer) *Controller {

	// Create event broadcaster
	// Add cm-operator types to the default Kubernetes Scheme so Events can be
	// logged for cm-operator types.
	utilruntime.Must(cmscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:             kubeclientset,
		cmclientset:               cmclientset,
		clusterRolesLister:        clusterRoleInformer.Lister(),
		clusterRoleBindingsLister: clusterRoleBindingInformer.Lister(),
		configmapsLister:          configmapInformer.Lister(),
		deploymentsLister:         deploymentInformer.Lister(),
		deploymentsSynced:         deploymentInformer.Informer().HasSynced,
		customMetricsLister:       customMetricInformer.Lister(),
		customMetricsSynced:       customMetricInformer.Informer().HasSynced,
		workqueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CustomMetrics"),
		recorder:                  recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when CustomMetric resources change
	customMetricInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCustomMetric,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueCustomMetric(new)
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a CustomMetric resource will enqueue that CustomMetric resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting CustomMetric controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.customMetricsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process CustomMetric resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// CustomMetric resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CustomMetric resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CustomMetric resource with this namespace/name
	customMetric, err := c.customMetricsLister.CustomMetrics(namespace).Get(name)
	if err != nil {
		// The CustomMetric resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("custommetric '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	resourceName := customMetric.Name + "-prometheus"

	// Create clusterrole
	_, err = c.clusterRolesLister.Get(resourceName)
	if errors.IsNotFound(err) {
		_, err = c.kubeclientset.RbacV1().ClusterRoles().Create(context.TODO(), newClusterRole(customMetric, &resourceName), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	// Create clusterrolebinding
	_, err = c.clusterRoleBindingsLister.Get(resourceName)
	if errors.IsNotFound(err) {
		_, err = c.kubeclientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), newClusterRoleBinding(customMetric, &resourceName), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	// Create prometheus configmap
	_, err = c.configmapsLister.ConfigMaps(customMetric.Namespace).Get(resourceName)
	if errors.IsNotFound(err) {
		_, err = c.kubeclientset.CoreV1().ConfigMaps(customMetric.Namespace).Create(context.TODO(), newConfigMap(customMetric, &resourceName), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	// Get the deployment with the name specified in CustomMetric.spec
	deployment, err := c.deploymentsLister.Deployments(customMetric.Namespace).Get(resourceName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(customMetric.Namespace).Create(context.TODO(), newDeployment(customMetric, &resourceName), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this CustomMetric resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(deployment, customMetric) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(customMetric, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// Finally, we update the status block of the CustomMetric resource to reflect the
	// current state of the world
	err = c.updateCustomMetricStatus(customMetric, deployment)
	if err != nil {
		return err
	}

	c.recorder.Event(customMetric, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateCustomMetricStatus(customMetric *cmv1alpha1.CustomMetric, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	customMetricCopy := customMetric.DeepCopy()

	customMetricCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// klog.Infof("%+v\n", customMetric)

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the CustomMetric resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.cmclientset.CmoperatorV1alpha1().CustomMetrics(customMetric.Namespace).Update(context.TODO(), customMetricCopy, metav1.UpdateOptions{})
	return err
}

// enqueueCustomMetric takes a CustomMetric resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than CustomMetric.
func (c *Controller) enqueueCustomMetric(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the CustomMetric resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that CustomMetric resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a CustomMetric, we should not do anything more
		// with it.
		if ownerRef.Kind != "CustomMetric" {
			return
		}

		customMetric, err := c.customMetricsLister.CustomMetrics(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of custommetric '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueCustomMetric(customMetric)
		return
	}
}

func newClusterRole(customMetric *cmv1alpha1.CustomMetric, resourceName *string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: *resourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customMetric, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
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

func newClusterRoleBinding(customMetric *cmv1alpha1.CustomMetric, resourceName *string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: *resourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customMetric, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     *resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: customMetric.Namespace,
			},
		},
	}
}

func newConfigMap(customMetric *cmv1alpha1.CustomMetric, resourceName *string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: *resourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customMetric, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
			},
		},
		Data: map[string]string{
			"prometheus.yml": `
scrape_configs:

- job_name: 'kubernetes-pods'

  kubernetes_sd_configs:
    - role: pod

  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_cmoperator_io_scrape]
      action: keep
      regex: true
    - source_labels:
      - __meta_kubernetes_pod_annotationpresent_cmoperator_io_path
      - __meta_kubernetes_pod_annotation_cmoperator_io_path
      action: replace
      target_label: __metrics_path__
      regex: true;(.+)
    - source_labels: [__address__, __meta_kubernetes_pod_annotation_cmoperator_io_port]
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
    - source_labels: [__meta_kubernetes_node_annotation_cmoperator_io_scrape]
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

// newDeployment creates a new Deployment for a CustomMetric resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CustomMetric resource that 'owns' it.
func newDeployment(customMetric *cmv1alpha1.CustomMetric, resourceName *string) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "prometheus-server",
		"controller": customMetric.Name,
	}

	sidecarArgs := []string{
		fmt.Sprintf("--stackdriver.project-id=%s", customMetric.Spec.Project),
		fmt.Sprintf("--stackdriver.kubernetes.cluster-name=%s", customMetric.Spec.Cluster),
		fmt.Sprintf("--stackdriver.kubernetes.location=%s", customMetric.Spec.Location),
		"--prometheus.wal-directory=/prometheus/wal",
		"--log.level=debug",
	}

	for _, m := range customMetric.Spec.Metrics {
		sidecarArgs = append(sidecarArgs, fmt.Sprintf("--include={__name__=~\"%s\"}", m))
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *resourceName,
			Namespace: customMetric.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customMetric, cmv1alpha1.SchemeGroupVersion.WithKind("CustomMetric")),
			},
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
										Name: *resourceName,
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
