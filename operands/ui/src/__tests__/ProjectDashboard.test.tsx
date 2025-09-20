import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import '@testing-library/jest-dom';
import ProjectDashboard from '../components/ProjectDashboard';
import * as apiService from '../services/api';

// Mock the API service
jest.mock('../services/api');
const mockApiService = apiService as jest.Mocked<typeof apiService>;

// Mock data
const mockProjects = [
  {
    projectKey: 'PROJ1',
    name: 'Project 1',
    status: 'Current',
    lastSyncTime: '2025-09-19T14:25:00Z',
    syncedIssueCount: 156,
    gitRepository: 'git@github.com:company/proj1-mirror.git'
  },
  {
    projectKey: 'PROJ2',
    name: 'Project 2',
    status: 'Syncing',
    lastSyncTime: '2025-09-19T14:20:00Z',
    syncedIssueCount: 89,
    gitRepository: 'git@github.com:company/proj2-mirror.git'
  },
  {
    projectKey: 'PROJ3',
    name: 'Project 3',
    status: 'Error',
    lastSyncTime: '2025-09-19T14:10:00Z',
    syncedIssueCount: 45,
    gitRepository: 'git@github.com:company/proj3-mirror.git'
  }
];

const renderProjectDashboard = () => {
  return render(
    <BrowserRouter>
      <ProjectDashboard />
    </BrowserRouter>
  );
};

describe('ProjectDashboard', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders loading state initially', () => {
    mockApiService.getProjects.mockImplementation(() => new Promise(() => {})); // Never resolves
    
    renderProjectDashboard();
    
    expect(screen.getByText('Loading projects...')).toBeInTheDocument();
  });

  test('renders projects list successfully', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      expect(screen.getByText('JIRA CDC Projects')).toBeInTheDocument();
      expect(screen.getByText('PROJ1')).toBeInTheDocument();
      expect(screen.getByText('PROJ2')).toBeInTheDocument();
      expect(screen.getByText('PROJ3')).toBeInTheDocument();
    });

    // Check project details
    expect(screen.getByText('Project 1')).toBeInTheDocument();
    expect(screen.getByText('156 issues')).toBeInTheDocument();
    expect(screen.getByText('89 issues')).toBeInTheDocument();
    expect(screen.getByText('45 issues')).toBeInTheDocument();
  });

  test('renders status badges correctly', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      // Check for status badges
      expect(screen.getByText('Current')).toBeInTheDocument();
      expect(screen.getByText('Syncing')).toBeInTheDocument();
      expect(screen.getByText('Error')).toBeInTheDocument();
    });
  });

  test('handles API error gracefully', async () => {
    mockApiService.getProjects.mockRejectedValueOnce(new Error('API Error'));
    
    renderProjectDashboard();
    
    await waitFor(() => {
      expect(screen.getByText('Error loading projects')).toBeInTheDocument();
      expect(screen.getByText('Failed to load projects. Please try again.')).toBeInTheDocument();
    });
  });

  test('refresh button works correctly', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      expect(screen.getByText('PROJ1')).toBeInTheDocument();
    });

    // Clear the mock to set up a second call
    mockApiService.getProjects.mockClear();
    mockApiService.getProjects.mockResolvedValueOnce([...mockProjects, {
      projectKey: 'PROJ4',
      name: 'Project 4',
      status: 'Current',
      lastSyncTime: '2025-09-19T14:30:00Z',
      syncedIssueCount: 25,
      gitRepository: 'git@github.com:company/proj4-mirror.git'
    }]);

    // Click refresh button
    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(screen.getByText('PROJ4')).toBeInTheDocument();
    });

    expect(mockApiService.getProjects).toHaveBeenCalledTimes(1);
  });

  test('project cards have correct links', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      const projectLink = screen.getByRole('link', { name: /proj1/i });
      expect(projectLink).toHaveAttribute('href', '/projects/PROJ1');
    });
  });

  test('displays last sync time correctly', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      // Should display formatted time (exact format depends on implementation)
      expect(screen.getByText(/14:25/)).toBeInTheDocument();
      expect(screen.getByText(/14:20/)).toBeInTheDocument();
      expect(screen.getByText(/14:10/)).toBeInTheDocument();
    });
  });

  test('empty state when no projects', async () => {
    mockApiService.getProjects.mockResolvedValueOnce([]);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      expect(screen.getByText('No projects found')).toBeInTheDocument();
      expect(screen.getByText('No JIRA CDC projects are currently configured.')).toBeInTheDocument();
    });
  });

  test('sync status icons are displayed', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      // Check for status-specific icons
      const currentIcon = screen.getByTestId('status-icon-current');
      const syncingIcon = screen.getByTestId('status-icon-syncing');
      const errorIcon = screen.getByTestId('status-icon-error');
      
      expect(currentIcon).toBeInTheDocument();
      expect(syncingIcon).toBeInTheDocument();
      expect(errorIcon).toBeInTheDocument();
    });
  });

  test('project cards are clickable', async () => {
    mockApiService.getProjects.mockResolvedValueOnce(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      const projectCard = screen.getByTestId('project-card-PROJ1');
      expect(projectCard).toBeInTheDocument();
      
      // Card should be clickable/have cursor pointer
      expect(projectCard).toHaveStyle('cursor: pointer');
    });
  });

  test('handles concurrent API calls correctly', async () => {
    let resolveFirst: (value: any) => void;
    let resolveSecond: (value: any) => void;
    
    const firstPromise = new Promise((resolve) => {
      resolveFirst = resolve;
    });
    const secondPromise = new Promise((resolve) => {
      resolveSecond = resolve;
    });

    mockApiService.getProjects
      .mockReturnValueOnce(firstPromise)
      .mockReturnValueOnce(secondPromise);
    
    renderProjectDashboard();
    
    // Trigger refresh before first call completes
    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);
    
    // Resolve second call first
    resolveSecond!(mockProjects);
    
    await waitFor(() => {
      expect(screen.getByText('PROJ1')).toBeInTheDocument();
    });
    
    // Resolve first call - should not overwrite the newer data
    resolveFirst!([]);
    
    // Wait a bit and ensure PROJ1 is still there
    await new Promise(resolve => setTimeout(resolve, 100));
    expect(screen.getByText('PROJ1')).toBeInTheDocument();
  });

  test('auto-refresh functionality', async () => {
    jest.useFakeTimers();
    
    mockApiService.getProjects.mockResolvedValue(mockProjects);
    
    renderProjectDashboard();
    
    await waitFor(() => {
      expect(screen.getByText('PROJ1')).toBeInTheDocument();
    });
    
    expect(mockApiService.getProjects).toHaveBeenCalledTimes(1);
    
    // Fast-forward 30 seconds (auto-refresh interval)
    jest.advanceTimersByTime(30000);
    
    await waitFor(() => {
      expect(mockApiService.getProjects).toHaveBeenCalledTimes(2);
    });
    
    jest.useRealTimers();
  });
});