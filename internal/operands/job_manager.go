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
	"time"

	"github.com/company/jira-cdc-operator/internal/common"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// JobType represents the type of sync job
type JobType string

const (
	JobTypeBootstrap      JobType = "bootstrap"
	JobTypeReconciliation JobType = "reconciliation"
	JobTypeMaintenance    JobType = "maintenance"
)

// JobManager manages sync job deployments
type JobManager struct {
	client.Client
	Scheme *runtime.Scheme
	resourceReconciler *common.ResourceReconciler
}

// NewJobManager creates a new job manager
func NewJobManager(client client.Client, scheme *runtime.Scheme) OperandManager {
	return &JobManager{
		Client: client,
		Scheme: scheme,
		resourceReconciler: &common.ResourceReconciler{Client: client},
	}
}

// Reconcile reconciles sync jobs based on the JiraCDC configuration
func (m *JobManager) Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Handle bootstrap job if needed
	if jiracdc.Spec.SyncTarget.InitialBootstrap {
		if err := m.reconcileBootstrapJob(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to reconcile bootstrap job: %w", err)
		}
	}

	// Handle scheduled reconciliation jobs
	if jiracdc.Spec.SyncTarget.Schedule != "" {
		if err := m.reconcileReconciliationJob(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to reconcile reconciliation job: %w", err)
		}
	}

	return nil
}

// GetStatus returns the current status of sync jobs
func (m *JobManager) GetStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (OperandStatus, error) {
	// Check bootstrap job status
	bootstrapJob, bootstrapExists := m.getJobStatus(ctx, jiracdc, JobTypeBootstrap)
	
	// Check reconciliation job status
	reconciliationJob, reconciliationExists := m.getJobStatus(ctx, jiracdc, JobTypeReconciliation)

	status := OperandStatus{
		Type:      "Jobs",
		Ready:     true,
		Available: true,
		Message:   "No active jobs",
	}

	// Determine overall status
	if bootstrapExists {
		if bootstrapJob.Status.Active > 0 {
			status.Message = "Bootstrap job running"
			status.Ready = false
		} else if bootstrapJob.Status.Failed > 0 {
			status.Message = "Bootstrap job failed"
			status.Ready = false
		} else if bootstrapJob.Status.Succeeded > 0 {
			status.Message = "Bootstrap job completed"
		}
	}

	if reconciliationExists {
		if reconciliationJob.Status.Active > 0 {
			if status.Message == "No active jobs" {
				status.Message = "Reconciliation job running"
			} else {
				status.Message += ", Reconciliation job running"
			}
			status.Ready = false
		}
	}

	return status, nil
}

// Delete deletes sync job resources
func (m *JobManager) Delete(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Delete bootstrap job
	bootstrapJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getJobName(jiracdc, JobTypeBootstrap),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, bootstrapJob); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete bootstrap job: %w", err)
	}

	// Delete reconciliation job
	reconciliationJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getJobName(jiracdc, JobTypeReconciliation),
			Namespace: jiracdc.Namespace,
		},
	}
	if err := m.Delete(ctx, reconciliationJob); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete reconciliation job: %w", err)
	}

	return nil
}

// CreateBootstrapJob creates a one-time bootstrap job
func (m *JobManager) CreateBootstrapJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	return m.createJob(ctx, jiracdc, JobTypeBootstrap, false)
}

// CreateReconciliationJob creates a reconciliation job
func (m *JobManager) CreateReconciliationJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	return m.createJob(ctx, jiracdc, JobTypeReconciliation, false)
}

// CreateMaintenanceJob creates a maintenance job
func (m *JobManager) CreateMaintenanceJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	return m.createJob(ctx, jiracdc, JobTypeMaintenance, false)
}

// reconcileBootstrapJob handles bootstrap job reconciliation
func (m *JobManager) reconcileBootstrapJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Check if bootstrap job already exists and completed
	jobName := m.getJobName(jiracdc, JobTypeBootstrap)
	existingJob := &batchv1.Job{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: jiracdc.Namespace,
	}, existingJob)

	if err == nil {
		// Job exists, check if it completed successfully
		if existingJob.Status.Succeeded > 0 {
			// Bootstrap completed, no need to create another
			return nil
		}
		if existingJob.Status.Failed > 0 {
			// Previous job failed, delete it and create a new one
			if err := m.Delete(ctx, existingJob); err != nil {
				return fmt.Errorf("failed to delete failed bootstrap job: %w", err)
			}
		} else {
			// Job is still running, leave it alone
			return nil
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get bootstrap job: %w", err)
	}

	// Create new bootstrap job
	return m.createJob(ctx, jiracdc, JobTypeBootstrap, false)
}

// reconcileReconciliationJob handles reconciliation job reconciliation
func (m *JobManager) reconcileReconciliationJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// For now, we create jobs on-demand rather than using CronJob
	// This could be enhanced to use CronJob for scheduled reconciliation
	return nil
}

// createJob creates a Kubernetes Job for sync operations
func (m *JobManager) createJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC, jobType JobType, forceRefresh bool) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getJobName(jiracdc, jobType),
			Namespace: jiracdc.Namespace,
		},
	}

	_, err := m.resourceReconciler.CreateOrUpdateResource(ctx, job, func() error {
		// Set labels
		if job.Labels == nil {
			job.Labels = make(map[string]string)
		}
		job.Labels["app"] = "jiracdc-sync"
		job.Labels["jiracdc.io/instance"] = jiracdc.Name
		job.Labels["jiracdc.io/component"] = "sync-job"
		job.Labels["jiracdc.io/job-type"] = string(jobType)

		// Set owner reference
		if err := controllerutil.SetControllerReference(jiracdc, job, m.Scheme); err != nil {
			return err
		}

		// Configure job spec
		backoffLimit := int32(3)
		activeDeadlineSeconds := int64(3600) // 1 hour timeout

		job.Spec = batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                     "jiracdc-sync",
						"jiracdc.io/instance":     jiracdc.Name,
						"jiracdc.io/component":    "sync-job",
						"jiracdc.io/job-type":     string(jobType),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "sync",
							Image: m.getSyncImage(jiracdc),
							Command: []string{
								"/usr/local/bin/jiracdc-sync",
								"--operation", string(jobType),
								"--instance", jiracdc.Name,
								"--namespace", jiracdc.Namespace,
							},
							Env: m.getSyncJobEnv(jiracdc, jobType, forceRefresh),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jira-credentials",
									MountPath: "/etc/jira-credentials",
									ReadOnly:  true,
								},
								{
									Name:      "git-credentials",
									MountPath: "/etc/git-credentials",
									ReadOnly:  true,
								},
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(256*1024*1024, resource.BinarySI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "jira-credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: jiracdc.Spec.JiraInstance.CredentialsSecret,
								},
							},
						},
						{
							Name: "git-credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: jiracdc.Spec.GitRepository.CredentialsSecret,
								},
							},
						},
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: resource.NewQuantity(1024*1024*1024, resource.BinarySI), // 1GB
								},
							},
						},
					},
					ServiceAccountName: "jiracdc-sync",
				},
			},
		}

		return nil
	})

	return err
}

// getSyncJobEnv returns environment variables for sync jobs
func (m *JobManager) getSyncJobEnv(jiracdc *jiradcdv1.JiraCDC, jobType JobType, forceRefresh bool) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "JIRACDC_INSTANCE_NAME",
			Value: jiracdc.Name,
		},
		{
			Name:  "JIRACDC_INSTANCE_NAMESPACE",
			Value: jiracdc.Namespace,
		},
		{
			Name:  "JIRACDC_JOB_TYPE",
			Value: string(jobType),
		},
		{
			Name:  "JIRA_URL",
			Value: jiracdc.Spec.JiraInstance.URL,
		},
		{
			Name:  "JIRA_PROJECT_KEY",
			Value: jiracdc.Spec.JiraInstance.ProjectKey,
		},
		{
			Name:  "GIT_REPOSITORY_URL",
			Value: jiracdc.Spec.GitRepository.URL,
		},
		{
			Name:  "GIT_BRANCH",
			Value: jiracdc.Spec.GitRepository.Branch,
		},
		{
			Name:  "WORKSPACE_DIR",
			Value: "/workspace",
		},
		{
			Name:  "JIRA_CREDENTIALS_PATH",
			Value: "/etc/jira-credentials",
		},
		{
			Name:  "GIT_CREDENTIALS_PATH",
			Value: "/etc/git-credentials",
		},
	}

	// Add job-specific environment variables
	switch jobType {
	case JobTypeBootstrap:
		env = append(env, corev1.EnvVar{
			Name:  "FORCE_REFRESH",
			Value: "true",
		})
		env = append(env, corev1.EnvVar{
			Name:  "BATCH_SIZE",
			Value: "50",
		})
	case JobTypeReconciliation:
		env = append(env, corev1.EnvVar{
			Name:  "FORCE_REFRESH",
			Value: fmt.Sprintf("%t", forceRefresh),
		})
		env = append(env, corev1.EnvVar{
			Name:  "ACTIVE_ISSUES_ONLY",
			Value: fmt.Sprintf("%t", jiracdc.Spec.SyncTarget.ActiveIssuesOnly),
		})
		if jiracdc.Spec.SyncTarget.IssueFilter != "" {
			env = append(env, corev1.EnvVar{
				Name:  "ISSUE_FILTER",
				Value: jiracdc.Spec.SyncTarget.IssueFilter,
			})
		}
	case JobTypeMaintenance:
		env = append(env, corev1.EnvVar{
			Name:  "MAINTENANCE_MODE",
			Value: "true",
		})
	}

	return env
}

// getJobStatus retrieves the status of a job
func (m *JobManager) getJobStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC, jobType JobType) (*batchv1.Job, bool) {
	job := &batchv1.Job{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      m.getJobName(jiracdc, jobType),
		Namespace: jiracdc.Namespace,
	}, job)

	if err != nil {
		return nil, false
	}

	return job, true
}

// Helper methods for resource naming
func (m *JobManager) getJobName(jiracdc *jiradcdv1.JiraCDC, jobType JobType) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s-%s", jiracdc.Name, string(jobType), timestamp)
}

func (m *JobManager) getSyncImage(jiracdc *jiradcdv1.JiraCDC) string {
	// Use image from spec or default
	if jiracdc.Spec.OperandImages.Sync != "" {
		return jiracdc.Spec.OperandImages.Sync
	}
	return "jiracdc/sync:latest"
}