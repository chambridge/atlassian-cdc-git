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

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// Manager defines the interface for managing JiraCDC operands
type Manager interface {
	// Reconcile ensures all operands are in the desired state
	Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// Cleanup removes all operands associated with the JiraCDC instance
	Cleanup(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// GetAPIStatus returns the current status of the API operand
	GetAPIStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error)

	// GetUIStatus returns the current status of the UI operand
	GetUIStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error)

	// CreateBootstrapJob creates a new bootstrap job
	CreateBootstrapJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// CreateReconciliationJob creates a new reconciliation job
	CreateReconciliationJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// CancelCurrentTask cancels the currently running task
	CancelCurrentTask(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error
}

// APIManager defines the interface for managing API operands
type APIManager interface {
	// Reconcile ensures the API operand is in the desired state
	Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// Delete removes the API operand
	Delete(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// GetStatus returns the current status of the API operand
	GetStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error)

	// IsEnabled checks if the API operand is enabled
	IsEnabled(jiracdc *jiradcdv1.JiraCDC) bool

	// GetDesiredReplicas returns the desired number of replicas
	GetDesiredReplicas(jiracdc *jiradcdv1.JiraCDC) int32
}

// UIManager defines the interface for managing UI operands
type UIManager interface {
	// Reconcile ensures the UI operand is in the desired state
	Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// Delete removes the UI operand
	Delete(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// GetStatus returns the current status of the UI operand
	GetStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error)

	// IsEnabled checks if the UI operand is enabled
	IsEnabled(jiracdc *jiradcdv1.JiraCDC) bool

	// GetDesiredReplicas returns the desired number of replicas
	GetDesiredReplicas(jiracdc *jiradcdv1.JiraCDC) int32
}

// JobManager defines the interface for managing sync jobs
type JobManager interface {
	// CreateBootstrapJob creates a bootstrap job for initial synchronization
	CreateBootstrapJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// CreateReconciliationJob creates a reconciliation job for ongoing synchronization
	CreateReconciliationJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// CreateMaintenanceJob creates a maintenance job for cleanup operations
	CreateMaintenanceJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error

	// GetCurrentJob returns the currently running job for the JiraCDC instance
	GetCurrentJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.TaskInfo, error)

	// CancelJob cancels a running job
	CancelJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC, jobID string) error

	// ListJobs returns all jobs for the JiraCDC instance
	ListJobs(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) ([]*jiradcdv1.TaskInfo, error)

	// CleanupCompletedJobs removes old completed jobs
	CleanupCompletedJobs(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error
}

// OperandManagerImpl is the concrete implementation of the Manager interface
type OperandManagerImpl struct {
	APIManager APIManager
	UIManager  UIManager
	JobManager JobManager
}

// NewOperandManager creates a new operand manager
func NewOperandManager(apiManager APIManager, uiManager UIManager, jobManager JobManager) Manager {
	return &OperandManagerImpl{
		APIManager: apiManager,
		UIManager:  uiManager,
		JobManager: jobManager,
	}
}

// Reconcile ensures all operands are in the desired state
func (m *OperandManagerImpl) Reconcile(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Reconcile API operand
	if m.APIManager.IsEnabled(jiracdc) {
		if err := m.APIManager.Reconcile(ctx, jiracdc); err != nil {
			return err
		}
	} else {
		if err := m.APIManager.Delete(ctx, jiracdc); err != nil {
			return err
		}
	}

	// Reconcile UI operand
	if m.UIManager.IsEnabled(jiracdc) {
		if err := m.UIManager.Reconcile(ctx, jiracdc); err != nil {
			return err
		}
	} else {
		if err := m.UIManager.Delete(ctx, jiracdc); err != nil {
			return err
		}
	}

	// Handle initial bootstrap if needed
	if jiracdc.Spec.SyncConfig != nil && 
	   jiracdc.Spec.SyncConfig.Bootstrap != nil && 
	   *jiracdc.Spec.SyncConfig.Bootstrap && 
	   jiracdc.Status.Phase == "Pending" {
		if err := m.JobManager.CreateBootstrapJob(ctx, jiracdc); err != nil {
			return err
		}
	}

	return nil
}

// Cleanup removes all operands associated with the JiraCDC instance
func (m *OperandManagerImpl) Cleanup(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Delete API operand
	if err := m.APIManager.Delete(ctx, jiracdc); err != nil {
		return err
	}

	// Delete UI operand
	if err := m.UIManager.Delete(ctx, jiracdc); err != nil {
		return err
	}

	// Cleanup completed jobs
	if err := m.JobManager.CleanupCompletedJobs(ctx, jiracdc); err != nil {
		return err
	}

	return nil
}

// GetAPIStatus returns the current status of the API operand
func (m *OperandManagerImpl) GetAPIStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error) {
	return m.APIManager.GetStatus(ctx, jiracdc)
}

// GetUIStatus returns the current status of the UI operand
func (m *OperandManagerImpl) GetUIStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (*jiradcdv1.OperandStatus, error) {
	return m.UIManager.GetStatus(ctx, jiracdc)
}

// CreateBootstrapJob creates a new bootstrap job
func (m *OperandManagerImpl) CreateBootstrapJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	return m.JobManager.CreateBootstrapJob(ctx, jiracdc)
}

// CreateReconciliationJob creates a new reconciliation job
func (m *OperandManagerImpl) CreateReconciliationJob(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	return m.JobManager.CreateReconciliationJob(ctx, jiracdc)
}

// CancelCurrentTask cancels the currently running task
func (m *OperandManagerImpl) CancelCurrentTask(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Get current job
	currentJob, err := m.JobManager.GetCurrentJob(ctx, jiracdc)
	if err != nil {
		return err
	}

	if currentJob != nil && (currentJob.Status == "pending" || currentJob.Status == "running") {
		return m.JobManager.CancelJob(ctx, jiracdc, currentJob.ID)
	}

	return nil
}