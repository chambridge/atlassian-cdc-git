package operands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	jiracdc "jiracdc-operator/api/v1"
)

func TestOperandManager_ReconcileAPI(t *testing.T) {
	// Create test JiraCDC resource
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jiracdc",
			Namespace: "default",
			UID:       "12345",
		},
		Spec: jiracdc.JiraCDCSpec{
			JiraInstance: jiracdc.JiraInstanceConfig{
				BaseURL:           "https://test.atlassian.net",
				CredentialsSecret: "jira-creds",
			},
			SyncTarget: jiracdc.SyncTargetConfig{
				Type:       "project",
				ProjectKey: "TEST",
			},
			GitRepository: jiracdc.GitRepositoryConfig{
				URL:               "git@github.com:test/repo.git",
				CredentialsSecret: "git-creds",
				Branch:            "main",
			},
			Operands: jiracdc.OperandsConfig{
				API: jiracdc.APIOperandConfig{
					Enabled:  true,
					Replicas: 2,
					Image:    "jiracdc/api:test",
				},
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, jiracdc.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(jiracdc).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test API operand reconciliation
	ctx := context.Background()
	err := manager.ReconcileAPI(ctx, jiracdc)
	require.NoError(t, err)

	// Verify deployment was created
	deployment := &appsv1.Deployment{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test-jiracdc",
		Namespace: "default",
	}, deployment)
	require.NoError(t, err)

	// Verify deployment configuration
	assert.Equal(t, int32(2), *deployment.Spec.Replicas)
	assert.Equal(t, "jiracdc/api:test", deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "jiracdc-api", deployment.Spec.Template.Spec.Containers[0].Name)

	// Verify environment variables
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "JIRA_BASE_URL",
		Value: "https://test.atlassian.net",
	})
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "PROJECT_KEY",
		Value: "TEST",
	})
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "GIT_REPOSITORY_URL",
		Value: "git@github.com:test/repo.git",
	})

	// Verify owner reference
	assert.Len(t, deployment.OwnerReferences, 1)
	assert.Equal(t, "JiraCDC", deployment.OwnerReferences[0].Kind)
	assert.Equal(t, jiracdc.Name, deployment.OwnerReferences[0].Name)

	// Verify service was created
	service := &corev1.Service{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test-jiracdc",
		Namespace: "default",
	}, service)
	require.NoError(t, err)

	// Verify service configuration
	assert.Equal(t, int32(8080), service.Spec.Ports[0].Port)
	assert.Equal(t, "http", service.Spec.Ports[0].Name)
	assert.Equal(t, map[string]string{
		"app.kubernetes.io/name":       "jiracdc-api",
		"app.kubernetes.io/instance":   "test-jiracdc",
		"app.kubernetes.io/component":  "api",
		"app.kubernetes.io/managed-by": "jiracdc-operator",
	}, service.Spec.Selector)
}

func TestOperandManager_ReconcileUI(t *testing.T) {
	// Create test JiraCDC resource
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jiracdc",
			Namespace: "default",
			UID:       "12345",
		},
		Spec: jiracdc.JiraCDCSpec{
			Operands: jiracdc.OperandsConfig{
				UI: jiracdc.UIOperandConfig{
					Enabled:  true,
					Replicas: 1,
					Image:    "jiracdc/ui:test",
				},
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, jiracdc.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(jiracdc).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test UI operand reconciliation
	ctx := context.Background()
	err := manager.ReconcileUI(ctx, jiracdc)
	require.NoError(t, err)

	// Verify deployment was created
	deployment := &appsv1.Deployment{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-ui-test-jiracdc",
		Namespace: "default",
	}, deployment)
	require.NoError(t, err)

	// Verify deployment configuration
	assert.Equal(t, int32(1), *deployment.Spec.Replicas)
	assert.Equal(t, "jiracdc/ui:test", deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "jiracdc-ui", deployment.Spec.Template.Spec.Containers[0].Name)

	// Verify environment variables for API endpoint discovery
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "REACT_APP_API_BASE_URL",
		Value: "http://jiracdc-api-test-jiracdc:8080/api/v1",
	})

	// Verify service was created
	service := &corev1.Service{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-ui-test-jiracdc",
		Namespace: "default",
	}, service)
	require.NoError(t, err)

	// Verify service configuration
	assert.Equal(t, int32(3000), service.Spec.Ports[0].Port)
	assert.Equal(t, "http", service.Spec.Ports[0].Name)
}

func TestOperandManager_ReconcileSyncJob(t *testing.T) {
	// Create test JiraCDC resource
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jiracdc",
			Namespace: "default",
			UID:       "12345",
		},
		Spec: jiracdc.JiraCDCSpec{
			JiraInstance: jiracdc.JiraInstanceConfig{
				BaseURL:           "https://test.atlassian.net",
				CredentialsSecret: "jira-creds",
			},
			SyncTarget: jiracdc.SyncTargetConfig{
				Type:       "project",
				ProjectKey: "TEST",
			},
			GitRepository: jiracdc.GitRepositoryConfig{
				URL:               "git@github.com:test/repo.git",
				CredentialsSecret: "git-creds",
				Branch:            "main",
			},
			SyncConfig: jiracdc.SyncConfig{
				Bootstrap: true,
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, jiracdc.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(jiracdc).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test sync job reconciliation
	ctx := context.Background()
	jobType := "bootstrap"
	err := manager.ReconcileSyncJob(ctx, jiracdc, jobType)
	require.NoError(t, err)

	// Verify job was created
	job := &batchv1.Job{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-sync-bootstrap-test-jiracdc",
		Namespace: "default",
	}, job)
	require.NoError(t, err)

	// Verify job configuration
	assert.Equal(t, "jiracdc/sync:latest", job.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "jiracdc-sync", job.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)

	// Verify environment variables
	envVars := job.Spec.Template.Spec.Containers[0].Env
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "SYNC_TYPE",
		Value: "bootstrap",
	})
	assert.Contains(t, envVars, corev1.EnvVar{
		Name:  "PROJECT_KEY",
		Value: "TEST",
	})

	// Verify owner reference
	assert.Len(t, job.OwnerReferences, 1)
	assert.Equal(t, "JiraCDC", job.OwnerReferences[0].Kind)
}

func TestOperandManager_GetOperandStatus(t *testing.T) {
	// Create test deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jiracdc-api-test",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/name":      "jiracdc-api",
				"app.kubernetes.io/instance":  "test",
				"app.kubernetes.io/component": "api",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &[]int32{2}[0],
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      2,
			ReadyReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test getting operand status
	ctx := context.Background()
	status, err := manager.GetOperandStatus(ctx, "default", "jiracdc-api-test")
	require.NoError(t, err)

	assert.True(t, status.Ready)
	assert.Equal(t, int32(2), status.Replicas)
	assert.Equal(t, int32(2), status.ReadyReplicas)
}

func TestOperandManager_DeleteOperand(t *testing.T) {
	// Create test deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jiracdc-api-test",
			Namespace: "default",
		},
	}

	// Create test service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jiracdc-api-test",
			Namespace: "default",
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, service).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test deleting operand
	ctx := context.Background()
	err := manager.DeleteOperand(ctx, "default", "jiracdc-api-test")
	require.NoError(t, err)

	// Verify deployment was deleted
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test",
		Namespace: "default",
	}, deployment)
	assert.True(t, errors.IsNotFound(err))

	// Verify service was deleted
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test",
		Namespace: "default",
	}, service)
	assert.True(t, errors.IsNotFound(err))
}

func TestOperandManager_DisabledOperand(t *testing.T) {
	// Create test JiraCDC resource with disabled API
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jiracdc",
			Namespace: "default",
		},
		Spec: jiracdc.JiraCDCSpec{
			Operands: jiracdc.OperandsConfig{
				API: jiracdc.APIOperandConfig{
					Enabled: false, // Disabled
				},
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, jiracdc.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(jiracdc).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test API operand reconciliation when disabled
	ctx := context.Background()
	err := manager.ReconcileAPI(ctx, jiracdc)
	require.NoError(t, err)

	// Verify no deployment was created
	deployment := &appsv1.Deployment{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test-jiracdc",
		Namespace: "default",
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestOperandManager_UpdateExistingDeployment(t *testing.T) {
	// Create existing deployment with old configuration
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jiracdc-api-test-jiracdc",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/name":      "jiracdc-api",
				"app.kubernetes.io/instance":  "test-jiracdc",
				"app.kubernetes.io/component": "api",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &[]int32{1}[0], // Old replica count
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "jiracdc-api",
							Image: "jiracdc/api:old", // Old image
						},
					},
				},
			},
		},
	}

	// Create test JiraCDC resource with updated configuration
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jiracdc",
			Namespace: "default",
			UID:       "12345",
		},
		Spec: jiracdc.JiraCDCSpec{
			JiraInstance: jiracdc.JiraInstanceConfig{
				BaseURL:           "https://test.atlassian.net",
				CredentialsSecret: "jira-creds",
			},
			SyncTarget: jiracdc.SyncTargetConfig{
				Type:       "project",
				ProjectKey: "TEST",
			},
			GitRepository: jiracdc.GitRepositoryConfig{
				URL:               "git@github.com:test/repo.git",
				CredentialsSecret: "git-creds",
				Branch:            "main",
			},
			Operands: jiracdc.OperandsConfig{
				API: jiracdc.APIOperandConfig{
					Enabled:  true,
					Replicas: 3,                 // Updated replica count
					Image:    "jiracdc/api:new", // Updated image
				},
			},
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, jiracdc.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(jiracdc, existingDeployment).
		Build()

	// Create operand manager
	manager := NewOperandManager(fakeClient, scheme)

	// Test API operand reconciliation (update)
	ctx := context.Background()
	err := manager.ReconcileAPI(ctx, jiracdc)
	require.NoError(t, err)

	// Verify deployment was updated
	deployment := &appsv1.Deployment{}
	err = fakeClient.Get(ctx, client.ObjectKey{
		Name:      "jiracdc-api-test-jiracdc",
		Namespace: "default",
	}, deployment)
	require.NoError(t, err)

	// Verify updated configuration
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
	assert.Equal(t, "jiracdc/api:new", deployment.Spec.Template.Spec.Containers[0].Image)
}