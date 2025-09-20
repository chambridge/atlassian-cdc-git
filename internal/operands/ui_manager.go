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

	"github.com/company/jira-cdc-operator/internal/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// UIManager manages the UI operand deployment
type UIManager struct {
	client.Client
	Scheme *runtime.Scheme
	resourceReconciler *common.ResourceReconciler
}

// NewUIManager creates a new UI manager
func NewUIManager(client client.Client, scheme *runtime.Scheme) OperandManager {
	return &UIManager{
		Client: client,
		Scheme: scheme,
		resourceReconciler: &common.ResourceReconciler{Client: client},
	}
}

// Reconcile reconciles the UI operand
func (m *UIManager) Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Create or update deployment
	if err := m.reconcileDeployment(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile UI deployment: %w", err)
	}

	// Create or update service
	if err := m.reconcileService(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile UI service: %w", err)
	}

	// Create or update configmap for nginx configuration
	if err := m.reconcileConfigMap(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile UI configmap: %w", err)
	}

	return nil
}

// GetStatus returns the current status of the UI operand
func (m *UIManager) GetStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (OperandStatus, error) {
	deployment := &appsv1.Deployment{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      m.getUIDeploymentName(jiracdc),
		Namespace: jiracdc.Namespace,
	}, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			return OperandStatus{
				Type:      "UI",
				Ready:     false,
				Available: false,
				Message:   "Deployment not found",
			}, nil
		}
		return OperandStatus{}, fmt.Errorf("failed to get UI deployment: %w", err)
	}

	// Check deployment status
	ready := deployment.Status.ReadyReplicas > 0
	available := deployment.Status.AvailableReplicas > 0

	status := OperandStatus{
		Type:      "UI",
		Ready:     ready,
		Available: available,
		Replicas:  deployment.Status.Replicas,
	}

	if ready && available {
		status.Message = "UI is running"
	} else {
		status.Message = "UI is not ready"
	}

	return status, nil
}

// Delete deletes the UI operand resources
func (m *UIManager) Delete(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Delete deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIDeploymentName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, deployment); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete UI deployment: %w", err)
	}

	// Delete service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIServiceName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete UI service: %w", err)
	}

	// Delete configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIConfigMapName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete UI configmap: %w", err)
	}

	return nil
}

// reconcileDeployment creates or updates the UI deployment
func (m *UIManager) reconcileDeployment(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIDeploymentName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := m.resourceReconciler.CreateOrUpdateResource(ctx, deployment, func() error {
		// Set labels
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		deployment.Labels["app"] = "jiracdc-ui"
		deployment.Labels["jiracdc.io/instance"] = jiracdc.Name
		deployment.Labels["jiracdc.io/component"] = "ui"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, deployment, m.Scheme); err != nil {
			return err
		}

		// Configure deployment spec
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas: &[]int32{1}[0], // Single replica for MVP
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                     "jiracdc-ui",
					"jiracdc.io/instance":     jiracdc.Name,
					"jiracdc.io/component":    "ui",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                     "jiracdc-ui",
						"jiracdc.io/instance":     jiracdc.Name,
						"jiracdc.io/component":    "ui",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "ui",
							Image: m.getUIImage(jiracdc),
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 3000,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "REACT_APP_API_BASE_URL",
									Value: fmt.Sprintf("http://%s:8080", m.getAPIServiceName(jiracdc)),
								},
								{
									Name:  "REACT_APP_JIRACDC_INSTANCE",
									Value: jiracdc.Name,
								},
								{
									Name:  "PORT",
									Value: "3000",
								},
								{
									Name:  "NODE_ENV",
									Value: "production",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "nginx-config",
									MountPath: "/etc/nginx/conf.d",
									ReadOnly:  true,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(64*1024*1024, resource.BinarySI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(256*1024*1024, resource.BinarySI),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "nginx-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.getUIConfigMapName(jiracdc),
									},
								},
							},
						},
					},
				},
			},
		}

		return nil
	})

	return err
}

// reconcileService creates or updates the UI service
func (m *UIManager) reconcileService(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIServiceName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := m.resourceReconciler.CreateOrUpdateResource(ctx, service, func() error {
		// Set labels
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}
		service.Labels["app"] = "jiracdc-ui"
		service.Labels["jiracdc.io/instance"] = jiracdc.Name
		service.Labels["jiracdc.io/component"] = "ui"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, service, m.Scheme); err != nil {
			return err
		}

		// Configure service spec
		service.Spec = corev1.ServiceSpec{
			Selector: map[string]string{
				"app":                     "jiracdc-ui",
				"jiracdc.io/instance":     jiracdc.Name,
				"jiracdc.io/component":    "ui",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       3000,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		}

		return nil
	})

	return err
}

// reconcileConfigMap creates or updates the UI nginx configuration
func (m *UIManager) reconcileConfigMap(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getUIConfigMapName(jiracdc),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := m.resourceReconciler.CreateOrUpdateResource(ctx, configMap, func() error {
		// Set labels
		if configMap.Labels == nil {
			configMap.Labels = make(map[string]string)
		}
		configMap.Labels["app"] = "jiracdc-ui"
		configMap.Labels["jiracdc.io/instance"] = jiracdc.Name
		configMap.Labels["jiracdc.io/component"] = "ui"

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, configMap, m.Scheme); err != nil {
			return err
		}

		// Configure nginx.conf for React SPA
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		configMap.Data["default.conf"] = `
server {
    listen 3000;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
    add_header Content-Security-Policy "default-src 'self' http: https: data: blob: 'unsafe-inline'" always;

    # Handle React Router
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API proxy to backend
    location /api/ {
        proxy_pass http://` + m.getAPIServiceName(jiracdc) + `:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Static assets caching
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
`

		return nil
	})

	return err
}

// Helper methods for resource naming
func (m *UIManager) getUIDeploymentName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-ui", jiracdc.Name)
}

func (m *UIManager) getUIServiceName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-ui", jiracdc.Name)
}

func (m *UIManager) getUIConfigMapName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-ui-config", jiracdc.Name)
}

func (m *UIManager) getAPIServiceName(jiracdc *jiradcdv1.JiraCDC) string {
	return fmt.Sprintf("%s-api", jiracdc.Name)
}

func (m *UIManager) getUIImage(jiracdc *jiradcdv1.JiraCDC) string {
	// Use image from spec or default
	if jiracdc.Spec.OperandImages.UI != "" {
		return jiracdc.Spec.OperandImages.UI
	}
	return "jiracdc/ui:latest"
}