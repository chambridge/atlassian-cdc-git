import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import '@testing-library/jest-dom';
import TaskMonitor from '../components/TaskMonitor';
import * as apiService from '../services/api';

// Mock the API service
jest.mock('../services/api');
const mockApiService = apiService as jest.Mocked<typeof apiService>;

// Mock data
const mockTasks = [
  {
    id: 'task-1',
    type: 'bootstrap',
    status: 'completed',
    projectKey: 'PROJ1',
    createdAt: '2025-09-19T14:00:00Z',
    startedAt: '2025-09-19T14:01:00Z',
    completedAt: '2025-09-19T14:15:00Z',
    progress: {
      totalItems: 100,
      processedItems: 100,
      percentComplete: 100.0,
      estimatedTimeRemaining: '0s'
    }
  },
  {
    id: 'task-2',
    type: 'reconciliation',
    status: 'running',
    projectKey: 'PROJ2',
    createdAt: '2025-09-19T14:10:00Z',
    startedAt: '2025-09-19T14:11:00Z',
    progress: {
      totalItems: 50,
      processedItems: 25,
      percentComplete: 50.0,
      estimatedTimeRemaining: '5m30s'
    }
  },
  {
    id: 'task-3',
    type: 'maintenance',
    status: 'failed',
    projectKey: 'PROJ1',
    createdAt: '2025-09-19T13:45:00Z',
    startedAt: '2025-09-19T13:46:00Z',
    progress: {
      totalItems: 20,
      processedItems: 10,
      percentComplete: 50.0
    }
  },
  {
    id: 'task-4',
    type: 'reconciliation',
    status: 'pending',
    projectKey: 'PROJ3',
    createdAt: '2025-09-19T14:20:00Z',
    progress: {
      totalItems: 0,
      processedItems: 0,
      percentComplete: 0.0
    }
  }
];

const mockTaskDetails = {
  ...mockTasks[1],
  configuration: {
    issueFilter: 'status != Done AND updated >= -7d',
    forceRefresh: false
  },
  createdBy: 'jiracdc-operator',
  errorMessage: '',
  operations: [
    {
      id: 'op-1',
      issueKey: 'PROJ2-123',
      operationType: 'update',
      status: 'completed',
      processedAt: '2025-09-19T14:12:00Z',
      gitOperation: {
        action: 'file updated',
        filePath: 'PROJ2-123.md',
        commitMessage: 'feat(PROJ2-123): update issue status',
        commitHash: 'abc123def'
      }
    },
    {
      id: 'op-2',
      issueKey: 'PROJ2-124',
      operationType: 'create',
      status: 'processing',
      gitOperation: {
        action: 'file created',
        filePath: 'PROJ2-124.md',
        commitMessage: 'feat(PROJ2-124): add new issue',
        commitHash: 'def456ghi'
      }
    }
  ]
};

const renderTaskMonitor = (props = {}) => {
  return render(
    <BrowserRouter>
      <TaskMonitor {...props} />
    </BrowserRouter>
  );
};

describe('TaskMonitor', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders loading state initially', () => {
    mockApiService.getTasks.mockImplementation(() => new Promise(() => {})); // Never resolves
    
    renderTaskMonitor();
    
    expect(screen.getByText('Loading tasks...')).toBeInTheDocument();
  });

  test('renders tasks list successfully', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('CDC Tasks')).toBeInTheDocument();
      expect(screen.getByText('task-1')).toBeInTheDocument();
      expect(screen.getByText('task-2')).toBeInTheDocument();
      expect(screen.getByText('task-3')).toBeInTheDocument();
      expect(screen.getByText('task-4')).toBeInTheDocument();
    });

    // Check task details
    expect(screen.getByText('bootstrap')).toBeInTheDocument();
    expect(screen.getByText('PROJ1')).toBeInTheDocument();
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  test('displays progress bars correctly', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      // Check for progress indicators
      expect(screen.getByText('100%')).toBeInTheDocument(); // Completed task
      expect(screen.getByText('50%')).toBeInTheDocument(); // Running task
    });
  });

  test('filters tasks by status', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-1')).toBeInTheDocument();
    });

    // Filter by running status
    mockApiService.getTasks.mockClear();
    mockApiService.getTasks.mockResolvedValueOnce([mockTasks[1]]); // Only running task
    
    const statusFilter = screen.getByRole('combobox', { name: /status/i });
    fireEvent.change(statusFilter, { target: { value: 'running' } });

    await waitFor(() => {
      expect(mockApiService.getTasks).toHaveBeenCalledWith({ status: 'running' });
    });
  });

  test('filters tasks by type', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-1')).toBeInTheDocument();
    });

    // Filter by bootstrap type
    mockApiService.getTasks.mockClear();
    mockApiService.getTasks.mockResolvedValueOnce([mockTasks[0]]); // Only bootstrap task
    
    const typeFilter = screen.getByRole('combobox', { name: /type/i });
    fireEvent.change(typeFilter, { target: { value: 'bootstrap' } });

    await waitFor(() => {
      expect(mockApiService.getTasks).toHaveBeenCalledWith({ type: 'bootstrap' });
    });
  });

  test('filters tasks by project', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-1')).toBeInTheDocument();
    });

    // Filter by project
    const projectFilter = screen.getByRole('textbox', { name: /project/i });
    fireEvent.change(projectFilter, { target: { value: 'PROJ1' } });
    fireEvent.blur(projectFilter);

    await waitFor(() => {
      expect(mockApiService.getTasks).toHaveBeenCalledWith({ projectKey: 'PROJ1' });
    });
  });

  test('shows task details modal', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    mockApiService.getTaskDetails.mockResolvedValueOnce(mockTaskDetails);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-2')).toBeInTheDocument();
    });

    // Click on a task to show details
    const taskRow = screen.getByTestId('task-row-task-2');
    fireEvent.click(taskRow);

    await waitFor(() => {
      expect(screen.getByText('Task Details')).toBeInTheDocument();
      expect(screen.getByText('jiracdc-operator')).toBeInTheDocument();
      expect(screen.getByText('status != Done AND updated >= -7d')).toBeInTheDocument();
    });

    // Check operations
    expect(screen.getByText('PROJ2-123')).toBeInTheDocument();
    expect(screen.getByText('PROJ2-124')).toBeInTheDocument();
    expect(screen.getByText('abc123def')).toBeInTheDocument();
  });

  test('cancels running task', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    mockApiService.cancelTask.mockResolvedValueOnce({ 
      taskId: 'task-2', 
      status: 'cancelled',
      message: 'Task cancelled successfully' 
    });
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-2')).toBeInTheDocument();
    });

    // Find and click cancel button for running task
    const cancelButton = screen.getByTestId('cancel-task-task-2');
    fireEvent.click(cancelButton);

    // Confirm cancellation
    const confirmButton = screen.getByRole('button', { name: /confirm/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockApiService.cancelTask).toHaveBeenCalledWith('task-2');
    });
  });

  test('handles API error gracefully', async () => {
    mockApiService.getTasks.mockRejectedValueOnce(new Error('API Error'));
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('Error loading tasks')).toBeInTheDocument();
      expect(screen.getByText('Failed to load tasks. Please try again.')).toBeInTheDocument();
    });
  });

  test('auto-refreshes task list', async () => {
    jest.useFakeTimers();
    
    mockApiService.getTasks.mockResolvedValue(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-1')).toBeInTheDocument();
    });
    
    expect(mockApiService.getTasks).toHaveBeenCalledTimes(1);
    
    // Fast-forward 10 seconds (auto-refresh interval)
    jest.advanceTimersByTime(10000);
    
    await waitFor(() => {
      expect(mockApiService.getTasks).toHaveBeenCalledTimes(2);
    });
    
    jest.useRealTimers();
  });

  test('displays estimated time remaining', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('5m30s remaining')).toBeInTheDocument();
    });
  });

  test('empty state when no tasks', async () => {
    mockApiService.getTasks.mockResolvedValueOnce([]);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('No tasks found')).toBeInTheDocument();
      expect(screen.getByText('No CDC tasks match the current filters.')).toBeInTheDocument();
    });
  });

  test('sorts tasks by creation time', async () => {
    const unsortedTasks = [...mockTasks].reverse();
    mockApiService.getTasks.mockResolvedValueOnce(unsortedTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      const taskRows = screen.getAllByTestId(/task-row-/);
      // Tasks should be sorted by creation time (newest first)
      expect(taskRows[0]).toHaveTextContent('task-4');
      expect(taskRows[1]).toHaveTextContent('task-2');
      expect(taskRows[2]).toHaveTextContent('task-1');
      expect(taskRows[3]).toHaveTextContent('task-3');
    });
  });

  test('shows only cancellable tasks have cancel button', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    
    renderTaskMonitor();
    
    await waitFor(() => {
      // Running and pending tasks should have cancel button
      expect(screen.getByTestId('cancel-task-task-2')).toBeInTheDocument(); // running
      expect(screen.getByTestId('cancel-task-task-4')).toBeInTheDocument(); // pending
      
      // Completed and failed tasks should not have cancel button
      expect(screen.queryByTestId('cancel-task-task-1')).not.toBeInTheDocument(); // completed
      expect(screen.queryByTestId('cancel-task-task-3')).not.toBeInTheDocument(); // failed
    });
  });

  test('handles task cancellation error', async () => {
    mockApiService.getTasks.mockResolvedValueOnce(mockTasks);
    mockApiService.cancelTask.mockRejectedValueOnce(new Error('Cancellation failed'));
    
    renderTaskMonitor();
    
    await waitFor(() => {
      expect(screen.getByText('task-2')).toBeInTheDocument();
    });

    const cancelButton = screen.getByTestId('cancel-task-task-2');
    fireEvent.click(cancelButton);

    const confirmButton = screen.getByRole('button', { name: /confirm/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(screen.getByText('Error cancelling task')).toBeInTheDocument();
    });
  });
});