/*
Copyright 2025.

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

package operands

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// APIManager manages the API operand deployment
type APIManager struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewAPIManager creates a new API manager
func NewAPIManager(client client.Client, scheme *runtime.Scheme) OperandManager {
	return &APIManager{
		Client: client,
		Scheme: scheme,
	}
}

// Reconcile reconciles the API operand
func (m *APIManager) Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Create or update deployment
	if err := m.reconcileDeployment(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile API deployment: %w", err)
	}

	// Create or update service
	if err := m.reconcileService(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile API service: %w", err)
	}

	// Create or update configmap
	if err := m.reconcileConfigMap(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile API configmap: %w", err)
	}

	return nil
}

// GetStatus returns the current status of the API operand
func (m *APIManager) GetStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (OperandStatus, error) {
	deployment := &appsv1.Deployment{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      m.getAPIDeploymentName(jiracdc),
		Namespace: jiracdc.Namespace,
	}, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			return OperandStatus{
				Type:      "API",
				Ready:     false,
				Available: false,
				Message:   "Deployment not found",
			}, nil
		}
		return OperandStatus{}, fmt.Errorf("failed to get API deployment: %w", err)
	}

	// Check deployment status
	ready := deployment.Status.ReadyReplicas > 0
	available := deployment.Status.AvailableReplicas > 0

	status := OperandStatus{
		Type:      "API",
		Ready:     ready,
		Available: available,
		Replicas:  deployment.Status.Replicas,
	}

	if ready && available {
		status.Message = "API is running"
	} else {
		status.Message = "API is not ready"
	}

	return status, nil
}

// Delete deletes the API operand resources
func (m *APIManager) Delete(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Delete deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIDeploymentName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, deployment); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete API deployment: %w", err)
	}

	// Delete service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIServiceName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete API service: %w", err)
	}

	// Delete configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIConfigMapName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete API configmap: %w", err)
	}

	return nil
}

// reconcileDeployment creates or updates the API deployment
func (m *APIManager) reconcileDeployment(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIDeploymentName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.Client, deployment, func() error {
		// Set labels
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		deployment.Labels["app"] = "jiracdc-api"
		deployment.Labels["jiracdc.io/instance"] = jiracdc.Name
		deployment.Labels["jiracdc.io/component"] = "api"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, deployment, m.Scheme); err != nil {
			return err
		}

		// Configure deployment spec
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas: &[]int32{1}[0], // Single replica for MVP
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                     "jiracdc-api",
					"jiracdc.io/instance":     jiracdc.Name,
					"jiracdc.io/component":    "api",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                     "jiracdc-api",
						"jiracdc.io/instance":     jiracdc.Name,
						"jiracdc.io/component":    "api",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: m.getAPIImage(jiracdc),
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "metrics",
									ContainerPort: 9090,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "JIRACDC_INSTANCE_NAME",
									Value: jiracdc.Name,
								},
								{
									Name:  "JIRACDC_INSTANCE_NAMESPACE",
									Value: jiracdc.Namespace,
								},
								{
									Name:  "JIRACDC_API_PORT",
									Value: "8080",
								},
								{
									Name:  "JIRACDC_METRICS_PORT",
									Value: "9090",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/jiracdc",
									ReadOnly:  true,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(128*1024*1024, resource.BinarySI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(512*1024*1024, resource.BinarySI),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.getAPIConfigMapName(jiracdc),
									},
								},
							},
						},
					},
					ServiceAccountName: "jiracdc-api",
				},
			},
		}

		return nil
	})

	return err
}

// reconcileService creates or updates the API service
func (m *APIManager) reconcileService(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIServiceName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.Client, service, func() error {
		// Set labels
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}
		service.Labels["app"] = "jiracdc-api"
		service.Labels["jiracdc.io/instance"] = jiracdc.Name
		service.Labels["jiracdc.io/component"] = "api"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, service, m.Scheme); err != nil {
			return err
		}

		// Configure service spec
		service.Spec = corev1.ServiceSpec{
			Selector: map[string]string{
				"app":                     "jiracdc-api",
				"jiracdc.io/instance":     jiracdc.Name,
				"jiracdc.io/component":    "api",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       9090,
					TargetPort: intstr.FromString("metrics"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		}

		return nil
	})

	return err
}

// reconcileConfigMap creates or updates the API configuration
func (m *APIManager) reconcileConfigMap(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getAPIConfigMapName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.Client, configMap, func() error {
		// Set labels
		if configMap.Labels == nil {
			configMap.Labels = make(map[string]string)
		}
		configMap.Labels["app"] = "jiracdc-api"
		configMap.Labels["jiracdc.io/instance"] = jiracdc.Name
		configMap.Labels["jiracdc.io/component"] = "api"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, configMap, m.Scheme); err != nil {
			return err
		}

		// Configure config data
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		// API configuration
		configMap.Data["config.yaml"] = fmt.Sprintf(`
api:
  port: 8080
  host: "0.0.0.0"
  timeout: 30s
  
metrics:
  port: 9090
  path: "/metrics"
  
jira:
  instance: %s
  credentialsSecret: %s
  
git:
  credentialsSecret: %s
  
logging:
  level: info
  format: json
`, 
			jiracdc.Spec.JiraInstance.URL,
			jiracdc.Spec.JiraInstance.CredentialsSecret,
			jiracdc.Spec.GitRepository.CredentialsSecret,
		)

		return nil
	})

	return err
}

// Helper methods for resource naming
func (m *APIManager) getAPIDeploymentName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-api", jiracdc.Name)
}

func (m *APIManager) getAPIServiceName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-api", jiracdc.Name)
}

func (m *APIManager) getAPIConfigMapName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-api-config", jiracdc.Name)
}

func (m *APIManager) getAPIImage(jiracdc *jiradcdv1.JiraCDC) string {
	// Use image from spec or default
	if jiracdc.Spec.OperandImages.API != "" {
		return jiracdc.Spec.OperandImages.API
	}
	return "jiracdc/api:latest"
}