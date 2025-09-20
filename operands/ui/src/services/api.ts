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

import axios, { AxiosInstance, AxiosResponse } from 'axios';

// API Response Types
export interface ProjectStatus {
  projectKey: string;
  instanceName: string;
  status: string;
  jiraUrl: string;
  gitRepository: string;
  lastSync?: string;
  nextSync?: string;
  currentTask?: TaskInfo;
  operands: OperandStatus[];
  syncStats: ProjectSyncStats;
  configuration: ProjectConfiguration;
}

export interface TaskInfo {
  id: string;
  type: string;
  status: string;
  progress?: string;
}

export interface OperandStatus {
  type: string;
  ready: boolean;
  available: boolean;
  message: string;
  replicas: number;
}

export interface ProjectSyncStats {
  totalIssues: number;
  syncedIssues: number;
  failedIssues: number;
  lastSyncTime: string;
  syncDuration: string;
  averageIssueTime: string;
}

export interface ProjectConfiguration {
  activeIssuesOnly: boolean;
  issueFilter: string;
  schedule: string;
  batchSize: number;
  maxRetries: number;
}

export interface TaskResponse {
  id: string;
  type: string;
  status: string;
  projectKey: string;
  startedAt: string;
  completedAt?: string;
  errorMessage?: string;
  progress?: TaskProgress;
  configuration: TaskConfiguration;
  createdBy: string;
  finalCommitHash?: string;
}

export interface TaskProgress {
  totalItems: number;
  processedItems: number;
  percentComplete: number;
  estimatedTimeRemaining?: string;
}

export interface TaskConfiguration {
  issueFilter?: string;
  forceRefresh: boolean;
  activeIssuesOnly: boolean;
  batchSize: number;
  maxRetries: number;
}

export interface IssueResponse {
  key: string;
  summary: string;
  description: string;
  status: string;
  issueType: string;
  priority: string;
  assignee?: string;
  reporter?: string;
  labels: string[];
  components: string[];
  created: string;
  updated: string;
  projectKey: string;
  syncStatus: IssueSyncStatus;
  links: IssueLinks;
  metadata?: Record<string, string>;
}

export interface IssueSyncStatus {
  lastSynced?: string;
  syncedVersion: string;
  gitFilePath: string;
  commitHash?: string;
  status: string;
  errorMessage?: string;
}

export interface IssueLinks {
  jiraUrl: string;
  gitUrl?: string;
}

export interface HealthResponse {
  status: string;
  timestamp: string;
  uptime: string;
  version: string;
  components: ComponentHealth[];
  summary: HealthSummary;
}

export interface ComponentHealth {
  name: string;
  status: string;
  message?: string;
  lastChecked: string;
  details?: Record<string, string>;
  responseTime?: string;
}

export interface HealthSummary {
  healthyComponents: number;
  degradedComponents: number;
  unhealthyComponents: number;
  totalComponents: number;
}

// Request Types
export interface SyncRequest {
  type: string;
  forceRefresh: boolean;
  issueFilter?: string;
  batchSize?: number;
}

export interface CancelTaskRequest {
  reason?: string;
}

// API Client Configuration
interface ApiConfig {
  baseURL: string;
  timeout: number;
  retries: number;
}

class ApiService {
  private client: AxiosInstance;
  private config: ApiConfig;

  constructor(config?: Partial<ApiConfig>) {
    this.config = {
      baseURL: config?.baseURL || '/api/v1',
      timeout: config?.timeout || 30000,
      retries: config?.retries || 3,
    };

    this.client = axios.create({
      baseURL: this.config.baseURL,
      timeout: this.config.timeout,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors(): void {
    // Request interceptor
    this.client.interceptors.request.use(
      (config) => {
        // Add authentication headers if available
        const token = localStorage.getItem('auth_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor
    this.client.interceptors.response.use(
      (response) => response,
      async (error) => {
        if (error.response?.status === 401) {
          // Handle authentication errors
          localStorage.removeItem('auth_token');
          window.location.href = '/login';
        }
        return Promise.reject(error);
      }
    );
  }

  // Project APIs
  async getProjects(namespace?: string): Promise<{ projects: ProjectStatus[]; total: number }> {
    const params = namespace ? { namespace } : {};
    const response: AxiosResponse<{ projects: ProjectStatus[]; total: number }> = 
      await this.client.get('/projects', { params });
    return response.data;
  }

  async getProject(projectKey: string, namespace?: string): Promise<ProjectStatus> {
    const params = namespace ? { namespace } : {};
    const response: AxiosResponse<ProjectStatus> = 
      await this.client.get(`/projects/${projectKey}`, { params });
    return response.data;
  }

  async syncProject(projectKey: string, request: SyncRequest, namespace?: string): Promise<{ message: string; task: TaskInfo }> {
    const params = namespace ? { namespace } : {};
    const response: AxiosResponse<{ message: string; task: TaskInfo }> = 
      await this.client.post(`/projects/${projectKey}/sync`, request, { params });
    return response.data;
  }

  async getProjectHealth(projectKey: string, namespace?: string): Promise<Record<string, any>> {
    const params = namespace ? { namespace } : {};
    const response: AxiosResponse<Record<string, any>> = 
      await this.client.get(`/projects/${projectKey}/health`, { params });
    return response.data;
  }

  async getProjectMetrics(projectKey: string, namespace?: string): Promise<Record<string, any>> {
    const params = namespace ? { namespace } : {};
    const response: AxiosResponse<Record<string, any>> = 
      await this.client.get(`/projects/${projectKey}/metrics`, { params });
    return response.data;
  }

  // Task APIs
  async getTasks(filters?: {
    projectKey?: string;
    status?: string;
    type?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ tasks: TaskResponse[]; total: number; offset: number; limit: number }> {
    const response: AxiosResponse<{ tasks: TaskResponse[]; total: number; offset: number; limit: number }> = 
      await this.client.get('/tasks', { params: filters });
    return response.data;
  }

  async getTask(taskId: string): Promise<TaskResponse> {
    const response: AxiosResponse<TaskResponse> = 
      await this.client.get(`/tasks/${taskId}`);
    return response.data;
  }

  async cancelTask(taskId: string, request?: CancelTaskRequest): Promise<{ message: string; taskId: string }> {
    const response: AxiosResponse<{ message: string; taskId: string }> = 
      await this.client.post(`/tasks/${taskId}/cancel`, request || {});
    return response.data;
  }

  async retryTask(taskId: string): Promise<{ message: string; taskId: string }> {
    const response: AxiosResponse<{ message: string; taskId: string }> = 
      await this.client.post(`/tasks/${taskId}/retry`);
    return response.data;
  }

  async deleteTask(taskId: string): Promise<{ message: string; taskId: string }> {
    const response: AxiosResponse<{ message: string; taskId: string }> = 
      await this.client.delete(`/tasks/${taskId}`);
    return response.data;
  }

  async getTaskProgress(taskId: string): Promise<Record<string, any>> {
    const response: AxiosResponse<Record<string, any>> = 
      await this.client.get(`/tasks/${taskId}/progress`);
    return response.data;
  }

  async getTaskLogs(taskId: string): Promise<{ taskId: string; logs: any[] }> {
    const response: AxiosResponse<{ taskId: string; logs: any[] }> = 
      await this.client.get(`/tasks/${taskId}/logs`);
    return response.data;
  }

  // Issue APIs
  async getIssues(filters: {
    projectKey: string;
    startAt?: number;
    maxResults?: number;
    status?: string;
    assignee?: string;
    search?: string;
  }): Promise<{
    issues: IssueResponse[];
    total: number;
    startAt: number;
    maxResults: number;
    projectKey: string;
  }> {
    const response: AxiosResponse<{
      issues: IssueResponse[];
      total: number;
      startAt: number;
      maxResults: number;
      projectKey: string;
    }> = await this.client.get('/issues', { params: filters });
    return response.data;
  }

  async getIssue(issueKey: string): Promise<IssueResponse> {
    const response: AxiosResponse<IssueResponse> = 
      await this.client.get(`/issues/${issueKey}`);
    return response.data;
  }

  async syncIssue(issueKey: string): Promise<{ message: string; issueKey: string; gitFilePath: string }> {
    const response: AxiosResponse<{ message: string; issueKey: string; gitFilePath: string }> = 
      await this.client.post(`/issues/${issueKey}/sync`);
    return response.data;
  }

  async getIssueHistory(issueKey: string): Promise<{ issueKey: string; history: any[] }> {
    const response: AxiosResponse<{ issueKey: string; history: any[] }> = 
      await this.client.get(`/issues/${issueKey}/history`);
    return response.data;
  }

  async getIssueComments(issueKey: string): Promise<{ issueKey: string; comments: any[] }> {
    const response: AxiosResponse<{ issueKey: string; comments: any[] }> = 
      await this.client.get(`/issues/${issueKey}/comments`);
    return response.data;
  }

  // Health APIs
  async getHealth(): Promise<HealthResponse> {
    const response: AxiosResponse<HealthResponse> = 
      await this.client.get('/health');
    return response.data;
  }

  async getReadiness(): Promise<{ ready: boolean; timestamp: string; checks: any[] }> {
    const response: AxiosResponse<{ ready: boolean; timestamp: string; checks: any[] }> = 
      await this.client.get('/ready');
    return response.data;
  }

  async getLiveness(): Promise<{ status: string; timestamp: string; uptime: string }> {
    const response: AxiosResponse<{ status: string; timestamp: string; uptime: string }> = 
      await this.client.get('/live');
    return response.data;
  }

  // Metrics APIs
  async getMetrics(): Promise<Record<string, any>> {
    const response: AxiosResponse<Record<string, any>> = 
      await this.client.get('/metrics');
    return response.data;
  }
}

// Export singleton instance
export const apiService = new ApiService();

// Export ApiService class for testing
export default ApiService;