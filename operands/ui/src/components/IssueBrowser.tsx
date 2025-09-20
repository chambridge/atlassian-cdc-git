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
  Label,
  LabelGroup,
  Flex,
  FlexItem,
  Grid,
  GridItem,
  Tabs,
  Tab,
  TabTitleText,
  List,
  ListItem,
  CodeBlock,
  CodeBlockCode
} from '@patternfly/react-core';
import {
  ExternalLinkAltIcon,
  SyncAltIcon,
  InfoCircleIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  TimesCircleIcon,
  GitIcon
} from '@patternfly/react-icons';
import { apiService, IssueResponse } from '@/services/api';

interface IssueBrowserProps {
  projectKey: string;
  autoRefresh?: boolean;
  refreshInterval?: number;
}

const IssueBrowser: React.FC<IssueBrowserProps> = ({ 
  projectKey, 
  autoRefresh = false, 
  refreshInterval = 60000 
}) => {
  const [issues, setIssues] = useState<IssueResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedIssue, setSelectedIssue] = useState<IssueResponse | null>(null);
  const [showIssueModal, setShowIssueModal] = useState(false);
  const [issueHistory, setIssueHistory] = useState<any[]>([]);
  const [issueComments, setIssueComments] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useState<string | number>(0);
  
  // Filters and pagination
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [assigneeFilter, setAssigneeFilter] = useState<string>('');
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(50);
  const [totalIssues, setTotalIssues] = useState(0);
  
  // Filter dropdowns
  const [statusFilterOpen, setStatusFilterOpen] = useState(false);
  const [assigneeFilterOpen, setAssigneeFilterOpen] = useState(false);

  const loadIssues = useCallback(async () => {
    try {
      setError(null);
      const filters: any = {
        projectKey,
        startAt: (page - 1) * perPage,
        maxResults: perPage
      };
      
      if (statusFilter) filters.status = statusFilter;
      if (assigneeFilter) filters.assignee = assigneeFilter;
      if (searchTerm) filters.search = searchTerm;
      
      const result = await apiService.getIssues(filters);
      setIssues(result.issues);
      setTotalIssues(result.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load issues');
    } finally {
      setLoading(false);
    }
  }, [projectKey, statusFilter, assigneeFilter, searchTerm, page, perPage]);

  const loadIssueDetails = async (issueKey: string) => {
    try {
      // Load issue history and comments in parallel
      const [historyResult, commentsResult] = await Promise.all([
        apiService.getIssueHistory(issueKey),
        apiService.getIssueComments(issueKey)
      ]);
      
      setIssueHistory(historyResult.history);
      setIssueComments(commentsResult.comments);
    } catch (err) {
      console.error('Failed to load issue details:', err);
      setIssueHistory([]);
      setIssueComments([]);
    }
  };

  useEffect(() => {
    loadIssues();
  }, [loadIssues]);

  useEffect(() => {
    if (!autoRefresh) return;
    
    const interval = setInterval(loadIssues, refreshInterval);
    return () => clearInterval(interval);
  }, [loadIssues, autoRefresh, refreshInterval]);

  const handleSyncIssue = async (issueKey: string) => {
    try {
      await apiService.syncIssue(issueKey);
      // Refresh issues after sync
      setTimeout(loadIssues, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync issue');
    }
  };

  const openIssueModal = async (issue: IssueResponse) => {
    setSelectedIssue(issue);
    setShowIssueModal(true);
    setActiveTab(0);
    await loadIssueDetails(issue.key);
  };

  const closeIssueModal = () => {
    setShowIssueModal(false);
    setSelectedIssue(null);
    setIssueHistory([]);
    setIssueComments([]);
    setActiveTab(0);
  };

  const getStatusBadge = (status: string) => {
    const statusMap: Record<string, { color: string }> = {
      'to do': { color: 'blue' },
      'in progress': { color: 'cyan' },
      'done': { color: 'green' },
      'closed': { color: 'green' },
      'resolved': { color: 'green' },
      'blocked': { color: 'red' },
      'cancelled': { color: 'orange' }
    };
    
    const config = statusMap[status.toLowerCase()] || { color: 'grey' };
    return <Badge color={config.color as any}>{status}</Badge>;
  };

  const getPriorityBadge = (priority: string) => {
    const priorityMap: Record<string, { color: string }> = {
      'highest': { color: 'red' },
      'high': { color: 'orange' },
      'medium': { color: 'blue' },
      'low': { color: 'green' },
      'lowest': { color: 'grey' }
    };
    
    const config = priorityMap[priority.toLowerCase()] || { color: 'grey' };
    return <Badge color={config.color as any}>{priority}</Badge>;
  };

  const getSyncStatusIcon = (syncStatus: string) => {
    switch (syncStatus.toLowerCase()) {
      case 'synced':
        return <CheckCircleIcon color="green" />;
      case 'pending':
        return <ExclamationTriangleIcon color="orange" />;
      case 'failed':
        return <TimesCircleIcon color="red" />;
      default:
        return <InfoCircleIcon color="grey" />;
    }
  };

  const renderIssueActions = (issue: IssueResponse) => (
    <Split hasGutter>
      <SplitItem>
        <Button
          variant="plain"
          icon={<InfoCircleIcon />}
          onClick={() => openIssueModal(issue)}
          aria-label="View issue details"
        />
      </SplitItem>
      
      <SplitItem>
        <Button
          variant="plain"
          icon={<SyncAltIcon />}
          onClick={() => handleSyncIssue(issue.key)}
          aria-label="Sync issue"
        />
      </SplitItem>
      
      <SplitItem>
        <Button
          variant="plain"
          icon={<ExternalLinkAltIcon />}
          component="a"
          href={issue.links.jiraUrl}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Open in JIRA"
        />
      </SplitItem>
      
      {issue.links.gitUrl && (
        <SplitItem>
          <Button
            variant="plain"
            icon={<GitIcon />}
            component="a"
            href={issue.links.gitUrl}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Open in Git"
          />
        </SplitItem>
      )}
    </Split>
  );

  const renderIssueDetails = () => {
    if (!selectedIssue) return null;

    return (
      <Stack hasGutter>
        <StackItem>
          <Grid hasGutter>
            <GridItem lg={8}>
              <Stack hasGutter>
                <StackItem>
                  <Split hasGutter>
                    <SplitItem>
                      <Text component={TextVariants.h3}>
                        {selectedIssue.key}: {selectedIssue.summary}
                      </Text>
                    </SplitItem>
                    <SplitItem>
                      {getStatusBadge(selectedIssue.status)}
                    </SplitItem>
                  </Split>
                </StackItem>
                
                <StackItem>
                  <Text component={TextVariants.p}>
                    {selectedIssue.description || 'No description available'}
                  </Text>
                </StackItem>
              </Stack>
            </GridItem>
            
            <GridItem lg={4}>
              <Stack hasGutter>
                <StackItem>
                  <Text component={TextVariants.h6}>Issue Type:</Text>
                  <Badge>{selectedIssue.issueType}</Badge>
                </StackItem>
                
                <StackItem>
                  <Text component={TextVariants.h6}>Priority:</Text>
                  {getPriorityBadge(selectedIssue.priority)}
                </StackItem>
                
                <StackItem>
                  <Text component={TextVariants.h6}>Assignee:</Text>
                  <Text>{selectedIssue.assignee || 'Unassigned'}</Text>
                </StackItem>
                
                <StackItem>
                  <Text component={TextVariants.h6}>Reporter:</Text>
                  <Text>{selectedIssue.reporter || 'Unknown'}</Text>
                </StackItem>
                
                {selectedIssue.labels.length > 0 && (
                  <StackItem>
                    <Text component={TextVariants.h6}>Labels:</Text>
                    <LabelGroup>
                      {selectedIssue.labels.map((label, index) => (
                        <Label key={index} color="blue">{label}</Label>
                      ))}
                    </LabelGroup>
                  </StackItem>
                )}
                
                {selectedIssue.components.length > 0 && (
                  <StackItem>
                    <Text component={TextVariants.h6}>Components:</Text>
                    <LabelGroup>
                      {selectedIssue.components.map((component, index) => (
                        <Label key={index} color="green">{component}</Label>
                      ))}
                    </LabelGroup>
                  </StackItem>
                )}
              </Stack>
            </GridItem>
          </Grid>
        </StackItem>
        
        <StackItem>
          <Card isCompact>
            <CardTitle>Sync Status</CardTitle>
            <CardBody>
              <Split hasGutter>
                <SplitItem>
                  {getSyncStatusIcon(selectedIssue.syncStatus.status)}
                </SplitItem>
                <SplitItem isFilled>
                  <Stack>
                    <StackItem>
                      <Text component={TextVariants.small}>
                        <strong>Status:</strong> {selectedIssue.syncStatus.status}
                      </Text>
                    </StackItem>
                    <StackItem>
                      <Text component={TextVariants.small}>
                        <strong>Git File:</strong> {selectedIssue.syncStatus.gitFilePath}
                      </Text>
                    </StackItem>
                    {selectedIssue.syncStatus.lastSynced && (
                      <StackItem>
                        <Text component={TextVariants.small}>
                          <strong>Last Synced:</strong>{' '}
                          <Timestamp 
                            date={new Date(selectedIssue.syncStatus.lastSynced)} 
                            tooltip={TimestampTooltipVariant.default}
                          />
                        </Text>
                      </StackItem>
                    )}
                    {selectedIssue.syncStatus.errorMessage && (
                      <StackItem>
                        <Alert variant={AlertVariant.danger} title="Sync Error" isInline>
                          {selectedIssue.syncStatus.errorMessage}
                        </Alert>
                      </StackItem>
                    )}
                  </Stack>
                </SplitItem>
              </Split>
            </CardBody>
          </Card>
        </StackItem>
      </Stack>
    );
  };

  const renderHistory = () => (
    <Stack hasGutter>
      {issueHistory.length > 0 ? (
        issueHistory.map((item, index) => (
          <StackItem key={index}>
            <Card isCompact>
              <CardBody>
                <Split hasGutter>
                  <SplitItem>
                    <Timestamp 
                      date={new Date(item.timestamp)} 
                      tooltip={TimestampTooltipVariant.default}
                    />
                  </SplitItem>
                  <SplitItem isFilled>
                    <Text component={TextVariants.small}>
                      <strong>{item.author}</strong> changed <strong>{item.field}</strong> 
                      {item.from && <> from <em>{item.from}</em></>}
                      {item.to && <> to <em>{item.to}</em></>}
                    </Text>
                  </SplitItem>
                </Split>
              </CardBody>
            </Card>
          </StackItem>
        ))
      ) : (
        <StackItem>
          <Alert variant={AlertVariant.info} title="No history available" isInline />
        </StackItem>
      )}
    </Stack>
  );

  const renderComments = () => (
    <Stack hasGutter>
      {issueComments.length > 0 ? (
        issueComments.map((comment, index) => (
          <StackItem key={index}>
            <Card>
              <CardBody>
                <Stack hasGutter>
                  <StackItem>
                    <Split hasGutter>
                      <SplitItem>
                        <Text component={TextVariants.h6}>
                          {comment.author}
                        </Text>
                      </SplitItem>
                      <SplitItem>
                        <Timestamp 
                          date={new Date(comment.created)} 
                          tooltip={TimestampTooltipVariant.default}
                        />
                      </SplitItem>
                    </Split>
                  </StackItem>
                  <StackItem>
                    <Text component={TextVariants.p}>
                      {comment.body}
                    </Text>
                  </StackItem>
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        ))
      ) : (
        <StackItem>
          <Alert variant={AlertVariant.info} title="No comments available" isInline />
        </StackItem>
      )}
    </Stack>
  );

  if (loading && issues.length === 0) {
    return (
      <Card>
        <CardBody>
          <Flex justifyContent={{ default: 'justifyContentCenter' }}>
            <FlexItem>
              <Spinner size="lg" />
              <Text component={TextVariants.p}>Loading issues...</Text>
            </FlexItem>
          </Flex>
        </CardBody>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardTitle>
          Issue Browser - {projectKey}
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
                  placeholder="Search issues..."
                  value={searchTerm}
                  onChange={setSearchTerm}
                  onSearch={loadIssues}
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
                  <SelectOption value="To Do">To Do</SelectOption>
                  <SelectOption value="In Progress">In Progress</SelectOption>
                  <SelectOption value="Done">Done</SelectOption>
                  <SelectOption value="Closed">Closed</SelectOption>
                </Select>
              </ToolbarItem>
              
              <ToolbarItem>
                <SearchInput
                  placeholder="Filter by assignee..."
                  value={assigneeFilter}
                  onChange={setAssigneeFilter}
                  onSearch={loadIssues}
                  onClear={() => setAssigneeFilter('')}
                />
              </ToolbarItem>
              
              <ToolbarItem variant="pagination" align={{ default: 'alignRight' }}>
                <Pagination
                  itemCount={totalIssues}
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
          
          {/* Issues Table */}
          <Table aria-label="Issues table">
            <Thead>
              <Tr>
                <Th>Issue Key</Th>
                <Th>Summary</Th>
                <Th>Status</Th>
                <Th>Priority</Th>
                <Th>Assignee</Th>
                <Th>Sync Status</Th>
                <Th>Updated</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {issues.map((issue) => (
                <Tr key={issue.key}>
                  <Td>{issue.key}</Td>
                  <Td>
                    <Text component={TextVariants.small}>
                      {issue.summary.length > 50 
                        ? `${issue.summary.substring(0, 50)}...` 
                        : issue.summary}
                    </Text>
                  </Td>
                  <Td>{getStatusBadge(issue.status)}</Td>
                  <Td>{getPriorityBadge(issue.priority)}</Td>
                  <Td>{issue.assignee || 'Unassigned'}</Td>
                  <Td>
                    <Split hasGutter>
                      <SplitItem>
                        {getSyncStatusIcon(issue.syncStatus.status)}
                      </SplitItem>
                      <SplitItem>
                        <Text component={TextVariants.small}>
                          {issue.syncStatus.status}
                        </Text>
                      </SplitItem>
                    </Split>
                  </Td>
                  <Td>
                    <Timestamp 
                      date={new Date(issue.updated)} 
                      tooltip={TimestampTooltipVariant.default}
                    />
                  </Td>
                  <Td>{renderIssueActions(issue)}</Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
          
          {issues.length === 0 && !loading && (
            <Alert variant={AlertVariant.info} title="No issues found" isInline>
              {searchTerm || statusFilter || assigneeFilter ? 
                'No issues match the current filters.' : 
                'No issues found for this project.'}
            </Alert>
          )}
        </CardBody>
      </Card>

      {/* Issue Details Modal */}
      <Modal
        variant={ModalVariant.large}
        title="Issue Details"
        isOpen={showIssueModal}
        onClose={closeIssueModal}
      >
        {selectedIssue && (
          <Tabs
            activeKey={activeTab}
            onSelect={(_, tabIndex) => setActiveTab(tabIndex)}
          >
            <Tab
              eventKey={0}
              title={<TabTitleText>Details</TabTitleText>}
            >
              {renderIssueDetails()}
            </Tab>
            <Tab
              eventKey={1}
              title={<TabTitleText>History</TabTitleText>}
            >
              {renderHistory()}
            </Tab>
            <Tab
              eventKey={2}
              title={<TabTitleText>Comments</TabTitleText>}
            >
              {renderComments()}
            </Tab>
          </Tabs>
        )}
      </Modal>
    </>
  );
};

export default IssueBrowser;