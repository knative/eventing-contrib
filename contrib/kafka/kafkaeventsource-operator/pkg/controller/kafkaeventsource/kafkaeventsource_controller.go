package kafkaeventsource

import (
	"context"
	"log"
	"reflect"
	"sort"
	"strconv"

	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"k8s.io/client-go/rest"

	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/kafka/kafkaeventsource-operator/pkg/apis/sources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new KafkaEventSource Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKafkaEventSource{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kafkaeventsource-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KafkaEventSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.KafkaEventSource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner KafkaEventSource
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &sourcesv1alpha1.KafkaEventSource{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileKafkaEventSource{}

// ReconcileKafkaEventSource reconciles a KafkaEventSource object
type ReconcileKafkaEventSource struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        client.Client
	dynamicClient dynamic.Interface
	scheme        *runtime.Scheme
}

// InjectConfig implemnts an interface which means the K8s config gets injected
func (r *ReconcileKafkaEventSource) InjectConfig(c *rest.Config) error {
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}

// Reconcile contains the main reconcile loop - read the state of the cluster and make changes based on the KafkaEventSource.Spec
func (r *ReconcileKafkaEventSource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Reconciling KafkaEventSource %s/%s\n", request.Namespace, request.Name)

	// Fetch the KafkaEventSource
	kafkaEventSource := &sourcesv1alpha1.KafkaEventSource{}

	err := r.client.Get(context.TODO(), request.NamespacedName, kafkaEventSource)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	//Resolve the SinkURI
	sinkURI, err := sinks.GetSinkURI(r.dynamicClient, kafkaEventSource.Spec.Sink, kafkaEventSource.Namespace)
	if err != nil {
		log.Printf("Error resolving SinkURI: %s", err)
		return reconcile.Result{}, err
	}

	if kafkaEventSource.Status.SinkURI != sinkURI {
		log.Printf("Setting the SinkURI to %s\n", sinkURI)
		kafkaEventSource.Status.SinkURI = sinkURI
		err = r.client.Update(context.TODO(), kafkaEventSource)
		if err != nil {
			log.Printf("failed to update KafkaEventSource status:, %s", err)
			return reconcile.Result{}, err
		}
	}

	// Create a new deployment for this EventSource
	dep := deploymentForKafka(kafkaEventSource)

	// Set KafkaEventSource as the owner and controller
	if err := controllerutil.SetControllerReference(kafkaEventSource, dep, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Deployment already exists
	foundDeploymentList := &appsv1.DeploymentList{}
	deploymentLabelSelector := labels.SelectorFromSet(labelsForKafkaEventSource(kafkaEventSource.Name))
	deploymentListOps := &client.ListOptions{Namespace: kafkaEventSource.Namespace, LabelSelector: deploymentLabelSelector}
	err = r.client.List(context.TODO(), deploymentListOps, foundDeploymentList)
	if err != nil || len(foundDeploymentList.Items) < 1 {
		//deployment not found
		log.Printf("Creating a new Deployment %s/%s\n", dep.Namespace, dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Deployment created successfully - requeue
		return reconcile.Result{Requeue: true}, nil
	}

	found := foundDeploymentList.Items[0]

	// Check the config - Environment Variables and annotations
	if !reflect.DeepEqual(dep.Spec.Template, found.Spec.Template) {
		found.Spec.Template = dep.Spec.Template
		err := r.client.Update(context.TODO(), &found)
		if err != nil {
			log.Printf("failed to update Deployment configuration:, %s", err)
			return reconcile.Result{}, err
		}
	}

	// Ensure the deployment size is the same as the spec. Defaults to 1 if not specified
	size := kafkaEventSource.Spec.Replicas
	if size == nil {
		intendedSize := int32(1)
		size = &intendedSize
	}

	if *found.Spec.Replicas != *size {
		*found.Spec.Replicas = *size
		log.Printf("Setting replicas:%d", *size)
		err = r.client.Update(context.TODO(), &found)
		if err != nil {
			log.Printf("Failed to update Deployment: %s", found.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

	// Update the KafkaEventSource status with the pod names
	// List the pods for this event source's deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForKafkaEventSource(kafkaEventSource.Name))
	listOps := &client.ListOptions{Namespace: kafkaEventSource.Namespace, LabelSelector: labelSelector}
	err = r.client.List(context.TODO(), listOps, podList)
	if err != nil {
		log.Printf("Failed to list pods when reconciling KafkaEventSource: %s", kafkaEventSource.Name)
		return reconcile.Result{}, err
	}

	podNames := getPodNames(podList.Items)
	nodes := kafkaEventSource.Status.Nodes
	sort.Strings(podNames)
	sort.Strings(nodes)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, nodes) {
		kafkaEventSource.Status.Nodes = podNames
		log.Printf("Updating EventSource.Status.Nodes")
		err := r.client.Update(context.TODO(), kafkaEventSource)
		if err != nil {
			log.Printf("failed to update KafkaEventSource status:, %s", err)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// deploymentForKafka generates the Kubernetes deployment object for the EventSource
func deploymentForKafka(kes *sourcesv1alpha1.KafkaEventSource) *appsv1.Deployment {
	labels := labelsForKafkaEventSource(kes.Name)
	replicas := kes.Spec.Replicas
	envvars := getEnvVars(kes)

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: 	      kes.Name,
			Namespace:    kes.Namespace,
			Labels:       labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotationsForKafkaEventSource(kes),
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           "sjwoodman/kafkaeventsource:latest",
						Name:            "kafkaeventsource",
						ImagePullPolicy: "IfNotPresent",
						Env:             envvars,
					}},
				},
			},
		},
	}
	return dep
}

// getEnvVars adds the Environment Variables to the containers which represent the configuration of the Kafka client
func getEnvVars(kes *sourcesv1alpha1.KafkaEventSource) []corev1.EnvVar {

	var ev = []corev1.EnvVar{}

	addStrIfNotEmpty(&ev, &kes.Spec.Bootstrap, "KAFKA_BOOTSTRAP_SERVERS")
	addStrIfNotEmpty(&ev, &kes.Spec.Topic, "KAFKA_TOPIC")
	addStrIfNotEmpty(&ev, kes.Spec.KafkaVersion, "KAFKA_VERSION")
	addStrIfNotEmpty(&ev, &kes.Status.SinkURI, "TARGET")
	addStrIfNotEmpty(&ev, kes.Spec.ConsumerGroupID, "CONSUMER_GROUP_ID")
	addIntIfNotEmpty(&ev, kes.Spec.Net.MaxOpenRequests, "NET_MAX_OPEN_REQUESTS")
	addIntIfNotEmpty(&ev, kes.Spec.Net.KeepAlive, "NET_KEEPALIVE")
	addBoolIfNotEmpty(&ev, kes.Spec.Net.Sasl.Enable, "NET_SASL_ENABLED")
	addBoolIfNotEmpty(&ev, kes.Spec.Net.Sasl.Handshake, "NET_SASL_HANDSHAKE")
	addStrIfNotEmpty(&ev, kes.Spec.Net.Sasl.User, "NET_SASL_USER")
	addStrIfNotEmpty(&ev, kes.Spec.Net.Sasl.Password, "NET_SASL_PASSWORD")
	addIntIfNotEmpty(&ev, kes.Spec.Consumer.MaxWaitTime, "CONSUMER_MAX_WAIT_TIME")
	addIntIfNotEmpty(&ev, kes.Spec.Consumer.MaxProcessingTime, "CONSUMER_MAX_PROCESSING_TIME")
	addIntIfNotEmpty(&ev, kes.Spec.Consumer.Offsets.CommitInterval, "CONSUMER_OFFSETS_COMMIT_INTERVAL")
	addIntIfNotEmpty(&ev, kes.Spec.Consumer.Offsets.Retention, "CONSUMER_OFFSETS_RETENTION")
	addStrIfNotEmpty(&ev, kes.Spec.Consumer.Offsets.InitialOffset, "CONSUMER_OFFSETS_INITIAL")
	addIntIfNotEmpty(&ev, kes.Spec.Consumer.Offsets.Retry.Max, "CONSUMER_OFFSETS_RETRY_MAX")
	addIntIfNotEmpty(&ev, kes.Spec.ChannelBufferSize, "CHANNEL_BUFFER_SIZE")
	addStrIfNotEmpty(&ev, kes.Spec.Group.PartitionStrategy, "GROUP_PARTITION_STRATEGY")
	addIntIfNotEmpty(&ev, kes.Spec.Group.Session.Timeout, "GROUP_SESSION_TIMEOUT")

	return ev
}

func addStrIfNotEmpty(evs *[]corev1.EnvVar, yamlKey *string, evKey string) {

	if yamlKey != nil {
		log.Printf("Adding configuration %s -> %s", evKey, *yamlKey)
		*evs = append(*evs, corev1.EnvVar{
			Name:  evKey,
			Value: *yamlKey,
		})
	}
}

func addIntIfNotEmpty(evs *[]corev1.EnvVar, yamlKey *int64, evKey string) {

	if yamlKey != nil {
		log.Printf("Adding configuration %s -> %d", evKey, *yamlKey)
		*evs = append(*evs, corev1.EnvVar{
			Name:  evKey,
			Value: strconv.FormatInt(*yamlKey, 10),
		})
	}
}

func addBoolIfNotEmpty(evs *[]corev1.EnvVar, yamlKey *bool, evKey string) {

	if yamlKey != nil {
		log.Printf("Adding configuration %s -> %t", evKey, *yamlKey)
		*evs = append(*evs, corev1.EnvVar{
			Name:  evKey,
			Value: strconv.FormatBool(*yamlKey),
		})
	}
}

// labelsForKafkaEventSource returns the labels for selecting the resources
func labelsForKafkaEventSource(name string) map[string]string {
	return map[string]string{"app": "kafkaeventsource", "kafkaeventsource_cr": name}
}

// annotationsForKafkaEventSource returns the labels for selecting the resources
func annotationsForKafkaEventSource(kes *sourcesv1alpha1.KafkaEventSource) map[string]string {

	annotations := map[string]string{"sidecar.istio.io/inject": "true"}

	if kes.Spec.ExternalIPRanges != nil {
		log.Printf("Adding Istio exclude: %s", *kes.Spec.ExternalIPRanges)
		annotations["traffic.sidecar.istio.io/excludeOutboundIPRanges"] = *kes.Spec.ExternalIPRanges
	}

	return annotations
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
