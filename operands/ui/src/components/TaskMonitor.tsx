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

import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  CardBody,
  CardTitle,
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td,
  Badge,
  Button,
  Progress,
  ProgressSize,
  Modal,
  ModalVariant,
  Stack,
  StackItem,
  Text,
  TextVariants,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Select,
  SelectOption,
  SelectVariant,
  SearchInput,
  Pagination,
  PaginationVariant,
  Alert,
  AlertVariant,
  Spinner,
  Split,
  SplitItem,
  Timestamp,
  TimestampTooltipVariant,
  CodeBlock,
  CodeBlockCode
} from '@patternfly/react-core';
import {
  PlayIcon,
  StopIcon,
  RedoIcon,
  TrashIcon,
  InfoCircleIcon,
  ExternalLinkAltIcon
} from '@patternfly/react-icons';
import { apiService, TaskResponse } from '@/services/api';

interface TaskMonitorProps {
  projectKey?: string;
  autoRefresh?: boolean;
  refreshInterval?: number;
}

const TaskMonitor: React.FC<TaskMonitorProps> = ({ 
  projectKey, 
  autoRefresh = true, 
  refreshInterval = 10000 
}) => {
  const [tasks, setTasks] = useState<TaskResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<TaskResponse | null>(null);
  const [showTaskModal, setShowTaskModal] = useState(false);
  const [taskLogs, setTaskLogs] = useState<any[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  
  // Filters and pagination
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(20);
  const [totalTasks, setTotalTasks] = useState(0);
  
  // Filter dropdowns
  const [statusFilterOpen, setStatusFilterOpen] = useState(false);
  const [typeFilterOpen, setTypeFilterOpen] = useState(false);

  const loadTasks = useCallback(async () => {
    try {
      setError(null);
      const filters: any = {
        limit: perPage,
        offset: (page - 1) * perPage
      };
      
      if (projectKey) filters.projectKey = projectKey;
      if (statusFilter) filters.status = statusFilter;
      if (typeFilter) filters.type = typeFilter;
      
      const result = await apiService.getTasks(filters);
      
      // Filter by search term locally for simplicity
      let filteredTasks = result.tasks;
      if (searchTerm) {
        filteredTasks = result.tasks.filter(task => 
          task.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
          task.projectKey.toLowerCase().includes(searchTerm.toLowerCase())
        );
      }
      
      setTasks(filteredTasks);
      setTotalTasks(result.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load tasks');
    } finally {
      setLoading(false);
    }
  }, [projectKey, statusFilter, typeFilter, searchTerm, page, perPage]);

  const loadTaskLogs = async (taskId: string) => {
    try {
      setLogsLoading(true);
      const result = await apiService.getTaskLogs(taskId);
      setTaskLogs(result.logs);
    } catch (err) {
      console.error('Failed to load task logs:', err);
      setTaskLogs([]);
    } finally {
      setLogsLoading(false);
    }
  };

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    if (!autoRefresh) return;
    
    const interval = setInterval(loadTasks, refreshInterval);
    return () => clearInterval(interval);
  }, [loadTasks, autoRefresh, refreshInterval]);

  const handleTaskAction = async (taskId: string, action: 'cancel' | 'retry' | 'delete') => {
    try {
      switch (action) {
        case 'cancel':
          await apiService.cancelTask(taskId);
          break;
        case 'retry':
          await apiService.retryTask(taskId);
          break;
        case 'delete':
          await apiService.deleteTask(taskId);
          break;
      }
      
      // Refresh tasks after action
      setTimeout(loadTasks, 1000);
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${action} task`);
    }
  };

  const openTaskModal = async (task: TaskResponse) => {
    setSelectedTask(task);
    setShowTaskModal(true);
    await loadTaskLogs(task.id);
  };

  const closeTaskModal = () => {
    setShowTaskModal(false);
    setSelectedTask(null);
    setTaskLogs([]);
  };

  const getStatusBadge = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      pending: { color: 'blue', text: 'Pending' },
      running: { color: 'cyan', text: 'Running' },
      completed: { color: 'green', text: 'Completed' },
      failed: { color: 'red', text: 'Failed' },
      cancelled: { color: 'orange', text: 'Cancelled' }
    };
    
    const config = statusMap[status] || { color: 'grey', text: status };
    return <Badge color={config.color as any}>{config.text}</Badge>;
  };

  const getTaskTypeLabel = (type: string) => {
    const typeMap: Record<string, string> = {
      bootstrap: 'Bootstrap',
      reconciliation: 'Reconciliation',
      maintenance: 'Maintenance'
    };
    return typeMap[type] || type;
  };

  const canCancelTask = (task: TaskResponse) => {
    return task.status === 'pending' || task.status === 'running';
  };

  const canRetryTask = (task: TaskResponse) => {
    return task.status === 'failed';
  };

  const canDeleteTask = (task: TaskResponse) => {
    return task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled';
  };

  const renderTaskActions = (task: TaskResponse) => (
    <Split hasGutter>
      <SplitItem>
        <Button
          variant="plain"
          icon={<InfoCircleIcon />}
          onClick={() => openTaskModal(task)}
          aria-label="View task details"
        />
      </SplitItem>
      
      {canCancelTask(task) && (
        <SplitItem>
          <Button
            variant="plain"
            icon={<StopIcon />}
            onClick={() => handleTaskAction(task.id, 'cancel')}
            aria-label="Cancel task"
          />
        </SplitItem>
      )}
      
      {canRetryTask(task) && (
        <SplitItem>
          <Button
            variant="plain"
            icon={<RedoIcon />}
            onClick={() => handleTaskAction(task.id, 'retry')}
            aria-label="Retry task"
          />
        </SplitItem>
      )}
      
      {canDeleteTask(task) && (
        <SplitItem>
          <Button
            variant="plain"
            icon={<TrashIcon />}
            onClick={() => handleTaskAction(task.id, 'delete')}
            aria-label="Delete task"
          />
        </SplitItem>
      )}
    </Split>
  );

  const renderProgress = (task: TaskResponse) => {
    if (!task.progress) return null;
    
    return (
      <Stack>
        <StackItem>
          <Progress
            value={task.progress.percentComplete}
            size={ProgressSize.sm}
          />
        </StackItem>
        <StackItem>
          <Text component={TextVariants.small}>
            {task.progress.processedItems}/{task.progress.totalItems} items
            {task.progress.estimatedTimeRemaining && (
              <> ({task.progress.estimatedTimeRemaining} remaining)</>
            )}
          </Text>
        </StackItem>
      </Stack>
    );
  };

  if (loading && tasks.length === 0) {
    return (
      <Card>
        <CardBody>
          <Split>
            <SplitItem>
              <Spinner size="lg" />
            </SplitItem>
            <SplitItem>
              <Text component={TextVariants.p}>Loading tasks...</Text>
            </SplitItem>
          </Split>
        </CardBody>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardTitle>
          Task Monitor
          {projectKey && <Text component={TextVariants.small}> - {projectKey}</Text>}
        </CardTitle>
        
        <CardBody>
          {error && (
            <Alert variant={AlertVariant.danger} title="Error" isInline>
              {error}
            </Alert>
          )}
          
          {/* Filters Toolbar */}
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <SearchInput
                  placeholder="Search tasks..."
                  value={searchTerm}
                  onChange={setSearchTerm}
                  onSearch={loadTasks}
                  onClear={() => setSearchTerm('')}
                />
              </ToolbarItem>
              
              <ToolbarItem>
                <Select
                  variant={SelectVariant.single}
                  onToggle={setStatusFilterOpen}
                  onSelect={(_, selection) => {
                    setStatusFilter(selection as string);
                    setStatusFilterOpen(false);
                  }}
                  selections={statusFilter}
                  isOpen={statusFilterOpen}
                  placeholderText="Filter by status"
                >
                  <SelectOption value="">All Statuses</SelectOption>
                  <SelectOption value="pending">Pending</SelectOption>
                  <SelectOption value="running">Running</SelectOption>
                  <SelectOption value="completed">Completed</SelectOption>
                  <SelectOption value="failed">Failed</SelectOption>
                  <SelectOption value="cancelled">Cancelled</SelectOption>
                </Select>
              </ToolbarItem>
              
              <ToolbarItem>
                <Select
                  variant={SelectVariant.single}
                  onToggle={setTypeFilterOpen}
                  onSelect={(_, selection) => {
                    setTypeFilter(selection as string);
                    setTypeFilterOpen(false);
                  }}
                  selections={typeFilter}
                  isOpen={typeFilterOpen}
                  placeholderText="Filter by type"
                >
                  <SelectOption value="">All Types</SelectOption>
                  <SelectOption value="bootstrap">Bootstrap</SelectOption>
                  <SelectOption value="reconciliation">Reconciliation</SelectOption>
                  <SelectOption value="maintenance">Maintenance</SelectOption>
                </Select>
              </ToolbarItem>
              
              <ToolbarItem variant="pagination" align={{ default: 'alignRight' }}>
                <Pagination
                  itemCount={totalTasks}
                  perPage={perPage}
                  page={page}
                  onSetPage={(_, newPage) => setPage(newPage)}
                  onPerPageSelect={(_, newPerPage) => {
                    setPerPage(newPerPage);
                    setPage(1);
                  }}
                  variant={PaginationVariant.top}
                  isCompact
                />
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          
          {/* Tasks Table */}
          <Table aria-label="Tasks table">
            <Thead>
              <Tr>
                <Th>Task ID</Th>
                <Th>Project</Th>
                <Th>Type</Th>
                <Th>Status</Th>
                <Th>Progress</Th>
                <Th>Started</Th>
                <Th>Duration</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {tasks.map((task) => (
                <Tr key={task.id}>
                  <Td>
                    <Text component={TextVariants.small}>
                      {task.id.substring(0, 8)}...
                    </Text>
                  </Td>
                  <Td>{task.projectKey}</Td>
                  <Td>{getTaskTypeLabel(task.type)}</Td>
                  <Td>{getStatusBadge(task.status)}</Td>
                  <Td>{renderProgress(task)}</Td>
                  <Td>
                    <Timestamp 
                      date={new Date(task.startedAt)} 
                      tooltip={TimestampTooltipVariant.default}
                    />
                  </Td>
                  <Td>
                    {task.completedAt ? (
                      <Text component={TextVariants.small}>
                        {Math.round(
                          (new Date(task.completedAt).getTime() - 
                           new Date(task.startedAt).getTime()) / 1000
                        )}s
                      </Text>
                    ) : task.status === 'running' ? (
                      <Text component={TextVariants.small}>
                        {Math.round(
                          (Date.now() - new Date(task.startedAt).getTime()) / 1000
                        )}s
                      </Text>
                    ) : (
                      '-'
                    )}
                  </Td>
                  <Td>{renderTaskActions(task)}</Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
          
          {tasks.length === 0 && !loading && (
            <Alert variant={AlertVariant.info} title="No tasks found" isInline>
              {searchTerm || statusFilter || typeFilter ? 
                'No tasks match the current filters.' : 
                'No tasks have been created yet.'}
            </Alert>
          )}
        </CardBody>
      </Card>

      {/* Task Details Modal */}
      <Modal
        variant={ModalVariant.large}
        title={`Task Details - ${selectedTask?.id}`}
        isOpen={showTaskModal}
        onClose={closeTaskModal}
      >
        {selectedTask && (
          <Stack hasGutter>
            <StackItem>
              <Split hasGutter>
                <SplitItem>
                  <Text component={TextVariants.h6}>Status:</Text>
                  {getStatusBadge(selectedTask.status)}
                </SplitItem>
                <SplitItem>
                  <Text component={TextVariants.h6}>Type:</Text>
                  {getTaskTypeLabel(selectedTask.type)}
                </SplitItem>
                <SplitItem>
                  <Text component={TextVariants.h6}>Project:</Text>
                  {selectedTask.projectKey}
                </SplitItem>
              </Split>
            </StackItem>
            
            {selectedTask.progress && (
              <StackItem>
                <Text component={TextVariants.h6}>Progress:</Text>
                {renderProgress(selectedTask)}
              </StackItem>
            )}
            
            {selectedTask.errorMessage && (
              <StackItem>
                <Alert variant={AlertVariant.danger} title="Error Message" isInline>
                  {selectedTask.errorMessage}
                </Alert>
              </StackItem>
            )}
            
            <StackItem>
              <Text component={TextVariants.h6}>Configuration:</Text>
              <CodeBlock>
                <CodeBlockCode>
                  {JSON.stringify(selectedTask.configuration, null, 2)}
                </CodeBlockCode>
              </CodeBlock>
            </StackItem>
            
            <StackItem>
              <Text component={TextVariants.h6}>Task Logs:</Text>
              {logsLoading ? (
                <Spinner size="md" />
              ) : (
                <CodeBlock>
                  <CodeBlockCode>
                    {taskLogs.length > 0 ? 
                      taskLogs.map(log => `[${log.timestamp}] ${log.level}: ${log.message}`).join('\n') :
                      'No logs available'
                    }
                  </CodeBlockCode>
                </CodeBlock>
              )}
            </StackItem>
          </Stack>
        )}
      </Modal>
    </>
  );
};

export default TaskMonitor;