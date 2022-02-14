/*
Copyright 2022.

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
	"errors"
	"fmt"
	"time"

	"github.com/jpillora/backoff"
	kapps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appv1 "github.com/tupyy/device-operator/api/v1"
)

// DeviceDeploymentReconciler reconciles a DeviceDeployment object
type DeviceDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var b *backoff.Backoff

func init() {
	b = &backoff.Backoff{
		//These are the defaults
		Min:    100 * time.Millisecond,
		Max:    60 * time.Second,
		Factor: 2,
		Jitter: false,
	}
}

//+kubebuilder:rbac:groups=app.device-operator.io,resources=devicedeployments;deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=app.device-operator.io,resources=devicedeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.device-operator.io,resources=devicedeployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DeviceDeployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DeviceDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var deviceDeployment appv1.DeviceDeployment
	if err := r.Get(ctx, req.NamespacedName, &deviceDeployment); err != nil {
		logger.Error(err, "failed to fetch device deployment")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// we suppose that device ids are in asc order.
	if deviceDeployment.Spec.EndDeviceID != nil {
		if *deviceDeployment.Spec.EndDeviceID < deviceDeployment.Spec.StartDeviceID {
			return ctrl.Result{}, fmt.Errorf("EndDeviceID '%d' must be superior to StartDeviceID '%d'", *deviceDeployment.Spec.EndDeviceID, deviceDeployment.Spec.StartDeviceID)
		}
	} else if deviceDeployment.Spec.Count == nil {
		return ctrl.Result{}, errors.New("either EndDeviceID or Count must be specified")
	}

	// compute deployments data
	deploymentsData := computeDeploymentData(deviceDeployment.Spec.StartDeviceID, deviceDeployment.Spec.EndDeviceID, deviceDeployment.Spec.DeviceCount, deviceDeployment.Spec.Count, deviceDeployment.Spec.MessagesFrequency)

	// set duration for scheduled result
	scheduledResult := ctrl.Result{RequeueAfter: b.Duration()}
	// reschedule set to true if we need to retry later on
	reschedule := false

	// list active jobs
	var deploymentList kapps.DeploymentList
	if err := r.List(ctx, &deploymentList, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		logger.Error(err, "failed to list child jobs")

		return scheduledResult, err
	}

	logger.V(1).Info("deployment count", "count", len(deploymentList.Items))

	// deploymentsData describe the deployments to be created.
	// keep only the new data
	for _, d := range deploymentList.Items {
		if d.DeletionTimestamp != nil {
			continue
		}

		dataID := d.Labels["dataID"]
		if _, exists := deploymentsData[dataID]; exists {
			delete(deploymentsData, dataID)

			continue
		}

		if err := r.Delete(ctx, &d, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "failed to delete old deployment")
			reschedule = true
		} else {
			logger.V(1).Info("deleted deployment", "deployment", d.Name)
		}
	}

	logger.V(1).Info("deployments to create", "deployments", deploymentsData)

	// create deployments
	for _, data := range deploymentsData {
		deployment, err := constructDeployment(&deviceDeployment, data)
		if err != nil {
			logger.Error(err, "failed to create deployment")
			reschedule = true

			continue
		}

		if err := ctrl.SetControllerReference(&deviceDeployment, deployment, r.Scheme); err != nil {
			logger.Error(err, "unable to set reference controller")
			reschedule = true

			continue
		}

		if err := r.Create(ctx, deployment); err != nil {
			logger.Error(err, "unable to create deployment", "deployment", deployment)
			reschedule = true

			continue
		}

		logger.V(1).Info("created device deployment", "deployment", deployment)
	}

	if reschedule {
		return scheduledResult, nil
	}

	return ctrl.Result{}, nil
}

// contruct a deployment which will simulate devices from startDeviceID to endDeviceID one by one.
func constructDeployment(deviceDeployment *appv1.DeviceDeployment, d deploymentData) (*kapps.Deployment, error) {
	// We want deployment names for a given nominal start time to have a deterministic name to avoid the same deployment being created twice
	name := fmt.Sprintf("%s-%d-%d", deviceDeployment.Name, d.StartDeviceID, d.EndDeviceID)

	deployment := &kapps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   deviceDeployment.Namespace,
		},
		Spec: *deviceDeployment.Spec.DeploymentTemplate.DeepCopy(),
	}

	// set env variable the startDeviceID and endDeviceID
	envVars := []corev1.EnvVar{
		{
			Name:  "START_DEVICE_ID",
			Value: fmt.Sprintf("%d", d.StartDeviceID),
		},
		{
			Name:  "END_DEVICE_ID",
			Value: fmt.Sprintf("%d", d.EndDeviceID),
		},
		{
			Name:  "MESSAGE_FREQUENCY",
			Value: fmt.Sprintf("%d", d.MessagesFrequency),
		},
	}

	deployment.Labels["dataID"] = d.Id()
	deployment.Spec.Template.ObjectMeta.Labels = map[string]string{"device": name}
	deployment.Spec.Template.Spec.Containers[0].Env = envVars
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"device": name}}

	return deployment, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = appv1.GroupVersion.String()
)

type deploymentData struct {
	StartDeviceID     int32
	EndDeviceID       int32
	MessagesFrequency int32
}

func (d deploymentData) Id() string {
	return fmt.Sprintf("%d-%d-%d", d.StartDeviceID, d.EndDeviceID, d.MessagesFrequency)
}

// computeDeploymentData returns a list with all deployments which are supposed to be running in the cluster.
func computeDeploymentData(startDeviceID int32, endDeviceID *int32, devicesPerDeployment int32, totalDevices *int32, messageFrequency int32) map[string]deploymentData {
	data := make(map[string]deploymentData)

	var _endDeviceID int32

	if endDeviceID == nil && totalDevices == nil {
		return data
	}

	if totalDevices != nil {
		_endDeviceID = startDeviceID + *totalDevices - 1
	} else if endDeviceID != nil {
		_endDeviceID = *endDeviceID
	}

	for {
		d := deploymentData{
			StartDeviceID:     startDeviceID,
			MessagesFrequency: messageFrequency,
		}

		if startDeviceID+devicesPerDeployment-1 < _endDeviceID {
			d.EndDeviceID = startDeviceID + devicesPerDeployment - 1
		} else {
			d.EndDeviceID = _endDeviceID
		}

		data[d.Id()] = d

		startDeviceID += devicesPerDeployment

		if startDeviceID > _endDeviceID {
			break
		}
	}

	return data
}

func exists(deployment kapps.Deployment, data deploymentData) bool {
	if val, found := deployment.Labels["dataID"]; found {
		return data.Id() == val
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kapps.Deployment{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the deployment object, extract the owner...
		deployment := rawObj.(*kapps.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "DeviceDeployment" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.DeviceDeployment{}).
		Owns(&kapps.Deployment{}).
		Complete(r)
}
