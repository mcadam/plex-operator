package plextranscodejob

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	plexv1alpha1 "github.com/mcadam/plex-operator/pkg/apis/plex/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_plextranscodejob")
// Queue to hold idle workers pod name
var workerQueue []string
// How many idle workers
var idleWorkers = 1
var namespace = os.Getenv("WATCH_NAMESPACE")
var component = os.Getenv("PLEX_OPERATOR_COMPONENT")

// Add creates a new PlexTranscodeJob Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePlexTranscodeJob{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	// Default to component operator
	if len(component) == 0 {
		component = "operator"
	}

	// Create a new PlexTranscode Job on start if executor component
	if component == "executor" {
		log.Info("Creating PlexTranscodeJob...")
		err := createPlexTranscodeJob(mgr.GetClient())
		if err != nil {
			log.Error(err, "Error creating PlexTranscodeJob")
			return err
		}
		// Need to sleep, if we exit directly the transcoder pod might not have
		// ack back to the server yet and the server will start a new transcoder
		time.Sleep(30 * time.Second)
		log.Info("Exiting executor...")
		os.Exit(0)
	}

	// Create a new controller
	c, err := controller.New("plextranscodejob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PlexTranscodeJob
	err = c.Watch(&source.Kind{Type: &plexv1alpha1.PlexTranscodeJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner PlexTranscodeJob
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &plexv1alpha1.PlexTranscodeJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePlexTranscodeJob implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePlexTranscodeJob{}

// ReconcilePlexTranscodeJob reconciles a PlexTranscodeJob object
type ReconcilePlexTranscodeJob struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PlexTranscodeJob object and makes changes based on the state read
// and what is in the PlexTranscodeJob.Spec
func (r *ReconcilePlexTranscodeJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PlexTranscodeJob")

	// Fetch the PlexTranscodeJob instance
	ptj := &plexv1alpha1.PlexTranscodeJob{}
	err := r.client.Get(context.TODO(), request.NamespacedName, ptj)
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
	// Display PlexTranscodeJob information status
	reqLogger.Info(
		"PlexTranscodeJob Status",
		"State", ptj.Status.State,
		"Transcoder", ptj.Status.Transcoder,
		"Error", ptj.Status.Error,
	)

	// Default to operator component
	if len(component) == 0 {
		component = "operator"
	}

	// Reconcile business logic according to component type
	switch component {
	case "executor":
		return r.ReconcileExecutor(ptj)
	case "transcoder":
		return r.ReconcileTranscoder(ptj)
	case "operator":
		return r.ReconcileOperator(ptj)
	}
	return reconcile.Result{}, nil
}

func (r *ReconcilePlexTranscodeJob) ReconcileExecutor(ptj *plexv1alpha1.PlexTranscodeJob) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (r *ReconcilePlexTranscodeJob) ReconcileTranscoder(ptj *plexv1alpha1.PlexTranscodeJob) (reconcile.Result, error) {
	// Recognize pood running as assigned to a PlexTranscodeJob
	if ptj.Status.State == plexv1alpha1.PlexTranscodeStateAssigned && ptj.Status.Transcoder == os.Getenv("POD_NAME") {
		log.Info("PlexTranscodeJob execute Transcoder")
		// Update ptj status
		ptj.Status.State = plexv1alpha1.PlexTranscodeStateStarted
		err := r.client.Update(context.TODO(), ptj)
		if err != nil {
			log.Error(err, "Failed to update PlexTranscodeJob status")
			return reconcile.Result{}, err
		}
		// Run Plex Transcoder
		ptj.Status.State, ptj.Status.Error = runPlexTranscoder(ptj)
		err = r.client.Update(context.TODO(), ptj)
		if err != nil {
			log.Error(err, "Failed to update PlexTranscodeJob status")
			return reconcile.Result{}, err
		}
		os.Exit(0)
	}
	return reconcile.Result{}, nil
}

func (r *ReconcilePlexTranscodeJob) ReconcileOperator(ptj *plexv1alpha1.PlexTranscodeJob) (reconcile.Result, error) {
	// Check idle transcoder pods queue
	log.Info("Idle workers queue status", "Queue.Length", len(workerQueue), "Queue.Pods", workerQueue)
	missing := idleWorkers - len(workerQueue)
	if missing > 0 {
		// Fetch the PlexMediaServer pod instance
		podList := &corev1.PodList{}
		labelSelector := labels.SelectorFromSet(labelsForPlexMediaServer())
		listOps := &client.ListOptions{Namespace: ptj.Namespace, LabelSelector: labelSelector}
		if err := r.client.List(context.TODO(), listOps, podList); err != nil {
			return reconcile.Result{}, err
		}

		// Create missing idle transcoder pods
		for i := 0; i < missing; i++ {
			pod := generateIdleTranscoderPod(ptj, podList)
			log.Info("Creating a new idle transcoder Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			err := r.client.Create(context.TODO(), pod)
			if err != nil {
				log.Error(err, "Failed to create new idle transcoder Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
				return reconcile.Result{}, err
			}
			workerQueue = append(workerQueue, pod.Name)
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Cleanup completed / failed jobs
	if ptj.Status.State == plexv1alpha1.PlexTranscodeStateCompleted || ptj.Status.State == plexv1alpha1.PlexTranscodeStateFailed {
		log.Info("PlexTranscodeJob completed or failed, up for deletion")
		err := r.client.Delete(context.TODO(), ptj)
		if err != nil {
			log.Error(err, "Failed to delete PlexTranscodeJob")
			return reconcile.Result{}, err
		}
	}

	// Assign idle transcoder Pod if ptj state is CREATED
	if ptj.Status.State == plexv1alpha1.PlexTranscodeStateCreated {
		var podName string
		podName= workerQueue[0]
		found := &corev1.Pod{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: ptj.Namespace}, found)
		// If not found we loop again to create new idle transcoder and retry
		if err != nil {
			return reconcile.Result{}, err
		}

		// Set PlexTranscodeJob instance as the owner and controller
		if err := controllerutil.SetControllerReference(ptj, found, r.scheme); err != nil {
			log.Error(err, "Failed to set controller reference")
			return reconcile.Result{}, err
		}
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			log.Error(err, "Failed to update Pod owner reference")
			return reconcile.Result{}, err
		}

		// Update ptj Status
		ptj.Status.Transcoder = podName
		ptj.Status.State = plexv1alpha1.PlexTranscodeStateAssigned
		err = r.client.Update(context.TODO(), ptj)
		if err != nil {
			log.Error(err, "Failed to update PlexTranscodeJob status")
			return reconcile.Result{}, err
		}
		workerQueue = workerQueue[1:]
	}
	return reconcile.Result{}, nil
}

// generateIdleTranscoderPod returns a Pod with Plex server and
// this controller to listen to events on PlexTranscodeJobs
func generateIdleTranscoderPod(ptj *plexv1alpha1.PlexTranscodeJob, pmsPodList *corev1.PodList) *corev1.Pod {
	// Get volumes from pms pod
	var volumes = []corev1.Volume{
		{
			Name: "shared",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	var volumeMounts = []corev1.VolumeMount{
		{
			Name:      "shared",
			MountPath: "/shared",
		},
	}
	var serviceAccount = "plex"

	for _, pod := range pmsPodList.Items {
		volumes = pod.Spec.Volumes
		volumeMounts = pod.Spec.Containers[0].VolumeMounts
		serviceAccount = pod.Spec.ServiceAccountName
	}

	labels := map[string]string{
		"app": ptj.Name,
		"component": "transcoder",
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "plex-transcoder-",
			Namespace: ptj.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount,
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: volumes,
			InitContainers: []corev1.Container{
				{
					Name:    "init-plex-transcoder",
					Image:   "mcadm/plex-operator:v0.0.1",
					Command: []string{
						"cp",
						"/plex-operator",
						"/shared/plex-operator",
					},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "shared",
							MountPath: "/shared",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "plex-transcoder",
					Image:   "linuxserver/plex:latest",
					Command: []string{
						"/shared/plex-operator",
					},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: volumeMounts,
					Env: []corev1.EnvVar{
						{
							Name: "PLEX_OPERATOR_COMPONENT",
							Value: "transcoder",
						},
						{
							Name: "POD_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									APIVersion: "v1",
									FieldPath:  "metadata.name",
								},
							},
						},
						{
							Name: "WATCH_NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									APIVersion: "v1",
									FieldPath:  "metadata.namespace",
								},
							},
						},
					},
				},
			},
		},
	}
}

func runPlexTranscoder(ptj *plexv1alpha1.PlexTranscodeJob) (plexv1alpha1.PlexTranscodeJobState, string) {
	args := ptj.Spec.Args[1:len(ptj.Spec.Args)]
	cmd := ptj.Spec.Args[0]
	command := exec.Command(cmd, args...)
	command.Dir = ptj.Spec.Cwd
	stderr, err := command.StderrPipe()
	if err != nil {
		return plexv1alpha1.PlexTranscodeStateFailed, err.Error()
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return plexv1alpha1.PlexTranscodeStateFailed, err.Error()
	}
	command.Env = ptj.Spec.Env
	err = command.Start()
	if err != nil {
		return plexv1alpha1.PlexTranscodeStateFailed, err.Error()
	}
	go io.Copy(os.Stderr, stderr)
	go io.Copy(os.Stdout, stdout)
	err = command.Wait()
	if err != nil {
		return plexv1alpha1.PlexTranscodeStateFailed, err.Error()
	}
	return plexv1alpha1.PlexTranscodeStateCompleted, ""
}

// rewriteEnv rewrites environment variables to be passed to the transcoder
func rewriteEnv(in []string) {
	// no changes needed
}

// rewriteArgs rewrites command args to be passed to the transcoder
func rewriteArgs(in []string) {
	pmsInternalAddress := os.Getenv("PMS_INTERNAL_ADDRESS")
	for i, v := range in {
		switch v {
		case "-progressurl", "-manifest_name", "-segment_list":
			in[i+1] = strings.Replace(in[i+1], "http://127.0.0.1:32400", pmsInternalAddress, 1)
		case "-loglevel", "-loglevel_plex":
			in[i+1] = "debug"
		}
	}
}

func generatePlexTranscodeJob(cwd string, env []string, args []string) *plexv1alpha1.PlexTranscodeJob {
	return &plexv1alpha1.PlexTranscodeJob{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "plex-transcoder-",
			Namespace: namespace,
		},
		Spec: plexv1alpha1.PlexTranscodeJobSpec{
			Args: args,
			Env: env,
			Cwd: cwd,
		},
		Status: plexv1alpha1.PlexTranscodeJobStatus{
			State: plexv1alpha1.PlexTranscodeStateCreated,
			Transcoder: "",
			Error: "",
		},
	}
}

func createPlexTranscodeJob(c client.Client) error {
	env := os.Environ()
	args := os.Args

	rewriteEnv(env)
	rewriteArgs(args)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ptj := generatePlexTranscodeJob(cwd, env, args)
	return c.Create(context.TODO(), ptj)
}
// labelsForApp creates a simple set of labels for App.
func labelsForPlexMediaServer() map[string]string {
	return map[string]string{"component": "executor"}
}
