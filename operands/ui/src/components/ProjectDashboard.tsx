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

import React, { useState, useEffect } from 'react';
import {
  Page,
  PageSection,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Title,
  Text,
  TextVariants,
  Button,
  Badge,
  Progress,
  ProgressSize,
  Split,
  SplitItem,
  Stack,
  StackItem,
  Alert,
  AlertVariant,
  Spinner,
  Flex,
  FlexItem,
  Label,
  LabelGroup,
  Timestamp,
  TimestampTooltipVariant
} from '@patternfly/react-core';
import {
  ExternalLinkAltIcon,
  SyncAltIcon,
  InfoCircleIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  TimesCircleIcon
} from '@patternfly/react-icons';
import { apiService, ProjectStatus, SyncRequest } from '@/services/api';

interface ProjectDashboardProps {
  projectKey: string;
  namespace?: string;
}

const ProjectDashboard: React.FC<ProjectDashboardProps> = ({ projectKey, namespace }) => {
  const [project, setProject] = useState<ProjectStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [syncMessage, setSyncMessage] = useState<string | null>(null);

  useEffect(() => {
    loadProject();
    // Set up polling for real-time updates
    const interval = setInterval(loadProject, 30000); // 30 seconds
    return () => clearInterval(interval);
  }, [projectKey, namespace]);

  const loadProject = async () => {
    try {
      setError(null);
      const projectData = await apiService.getProject(projectKey, namespace);
      setProject(projectData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load project');
    } finally {
      setLoading(false);
    }
  };

  const handleSync = async (type: 'bootstrap' | 'reconciliation') => {
    if (!project) return;

    try {
      setSyncing(true);
      setSyncMessage(null);
      
      const syncRequest: SyncRequest = {
        type,
        forceRefresh: type === 'bootstrap',
        batchSize: project.configuration.batchSize
      };

      const result = await apiService.syncProject(projectKey, syncRequest, namespace);
      setSyncMessage(`${type} sync started: ${result.message}`);
      
      // Refresh project data after a short delay
      setTimeout(loadProject, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to start ${type} sync`);
    } finally {
      setSyncing(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'ready':
        return <Badge color="green"><CheckCircleIcon /> Ready</Badge>;
      case 'pending':
        return <Badge color="blue"><InfoCircleIcon /> Pending</Badge>;
      case 'failed':
        return <Badge color="red"><TimesCircleIcon /> Failed</Badge>;
      default:
        return <Badge color="grey"><ExclamationTriangleIcon /> {status}</Badge>;
    }
  };

  const getOperandStatusIcon = (operand: { ready: boolean; available: boolean }) => {
    if (operand.ready && operand.available) {
      return <CheckCircleIcon color="green" />;
    } else if (operand.available) {
      return <ExclamationTriangleIcon color="orange" />;
    } else {
      return <TimesCircleIcon color="red" />;
    }
  };

  if (loading) {
    return (
      <PageSection>
        <Flex justifyContent={{ default: 'justifyContentCenter' }}>
          <FlexItem>
            <Spinner size="lg" />
            <Text component={TextVariants.p}>Loading project dashboard...</Text>
          </FlexItem>
        </Flex>
      </PageSection>
    );
  }

  if (error || !project) {
    return (
      <PageSection>
        <Alert variant={AlertVariant.danger} title="Error loading project">
          {error || 'Project not found'}
        </Alert>
      </PageSection>
    );
  }

  return (
    <Page>
      <PageSection variant="light">
        <Split hasGutter>
          <SplitItem isFilled>
            <Stack hasGutter>
              <StackItem>
                <Title headingLevel="h1" size="2xl">
                  {project.projectKey} Dashboard
                </Title>
                <Text component={TextVariants.p}>
                  Instance: {project.instanceName} | Status: {getStatusBadge(project.status)}
                </Text>
              </StackItem>
              
              {syncMessage && (
                <StackItem>
                  <Alert variant={AlertVariant.success} title="Sync Status" isInline>
                    {syncMessage}
                  </Alert>
                </StackItem>
              )}
            </Stack>
          </SplitItem>
          
          <SplitItem>
            <Stack hasGutter>
              <StackItem>
                <Button
                  variant="primary"
                  icon={<SyncAltIcon />}
                  isLoading={syncing}
                  isDisabled={syncing || project.currentTask?.status === 'running'}
                  onClick={() => handleSync('reconciliation')}
                >
                  Sync Changes
                </Button>
              </StackItem>
              <StackItem>
                <Button
                  variant="secondary"
                  icon={<SyncAltIcon />}
                  isLoading={syncing}
                  isDisabled={syncing || project.currentTask?.status === 'running'}
                  onClick={() => handleSync('bootstrap')}
                >
                  Full Bootstrap
                </Button>
              </StackItem>
            </Stack>
          </SplitItem>
        </Split>
      </PageSection>

      <PageSection>
        <Grid hasGutter>
          {/* Current Task Card */}
          {project.currentTask && (
            <GridItem span={12}>
              <Card>
                <CardTitle>Current Task</CardTitle>
                <CardBody>
                  <Stack hasGutter>
                    <StackItem>
                      <Split hasGutter>
                        <SplitItem>
                          <Text component={TextVariants.h6}>
                            {project.currentTask.type.toUpperCase()} - {project.currentTask.id}
                          </Text>
                        </SplitItem>
                        <SplitItem>
                          {getStatusBadge(project.currentTask.status)}
                        </SplitItem>
                      </Split>
                    </StackItem>
                    
                    {project.currentTask.progress && (
                      <StackItem>
                        <Progress 
                          value={parseFloat(project.currentTask.progress.replace('%', ''))}
                          title="Progress"
                          size={ProgressSize.lg}
                        />
                        <Text component={TextVariants.small}>
                          {project.currentTask.progress}
                        </Text>
                      </StackItem>
                    )}
                  </Stack>
                </CardBody>
              </Card>
            </GridItem>
          )}

          {/* Sync Statistics Card */}
          <GridItem lg={6} md={12}>
            <Card>
              <CardTitle>Sync Statistics</CardTitle>
              <CardBody>
                <Stack hasGutter>
                  <StackItem>
                    <Split hasGutter>
                      <SplitItem>
                        <Text component={TextVariants.h2}>
                          {project.syncStats.syncedIssues}
                        </Text>
                        <Text component={TextVariants.small}>
                          Synced Issues
                        </Text>
                      </SplitItem>
                      <SplitItem>
                        <Text component={TextVariants.h2}>
                          {project.syncStats.totalIssues}
                        </Text>
                        <Text component={TextVariants.small}>
                          Total Issues
                        </Text>
                      </SplitItem>
                    </Split>
                  </StackItem>
                  
                  <StackItem>
                    <Progress 
                      value={(project.syncStats.syncedIssues / project.syncStats.totalIssues) * 100}
                      title="Sync Progress"
                    />
                  </StackItem>
                  
                  {project.lastSync && (
                    <StackItem>
                      <Text component={TextVariants.small}>
                        Last sync: <Timestamp date={new Date(project.lastSync)} tooltip={TimestampTooltipVariant.default} />
                      </Text>
                    </StackItem>
                  )}
                  
                  <StackItem>
                    <Text component={TextVariants.small}>
                      Duration: {project.syncStats.syncDuration} | 
                      Avg per issue: {project.syncStats.averageIssueTime}
                    </Text>
                  </StackItem>
                </Stack>
              </CardBody>
            </Card>
          </GridItem>

          {/* Configuration Card */}
          <GridItem lg={6} md={12}>
            <Card>
              <CardTitle>Configuration</CardTitle>
              <CardBody>
                <Stack hasGutter>
                  <StackItem>
                    <LabelGroup categoryName="Filters">
                      <Label color={project.configuration.activeIssuesOnly ? 'green' : 'grey'}>
                        {project.configuration.activeIssuesOnly ? 'Active Issues Only' : 'All Issues'}
                      </Label>
                      {project.configuration.issueFilter && (
                        <Label color="blue">
                          Filter: {project.configuration.issueFilter}
                        </Label>
                      )}
                    </LabelGroup>
                  </StackItem>
                  
                  <StackItem>
                    <Text component={TextVariants.small}>
                      Batch Size: {project.configuration.batchSize} | 
                      Max Retries: {project.configuration.maxRetries}
                    </Text>
                  </StackItem>
                  
                  {project.configuration.schedule && (
                    <StackItem>
                      <Text component={TextVariants.small}>
                        Schedule: {project.configuration.schedule}
                      </Text>
                    </StackItem>
                  )}
                </Stack>
              </CardBody>
            </Card>
          </GridItem>

          {/* Operands Status Card */}
          <GridItem span={12}>
            <Card>
              <CardTitle>Operands Status</CardTitle>
              <CardBody>
                <Grid hasGutter>
                  {project.operands.map((operand, index) => (
                    <GridItem lg={4} md={6} key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Split hasGutter>
                            <SplitItem>
                              {getOperandStatusIcon(operand)}
                            </SplitItem>
                            <SplitItem isFilled>
                              <Stack>
                                <StackItem>
                                  <Text component={TextVariants.h6}>
                                    {operand.type.toUpperCase()}
                                  </Text>
                                </StackItem>
                                <StackItem>
                                  <Text component={TextVariants.small}>
                                    {operand.message}
                                  </Text>
                                </StackItem>
                                <StackItem>
                                  <Text component={TextVariants.small}>
                                    Replicas: {operand.replicas}
                                  </Text>
                                </StackItem>
                              </Stack>
                            </SplitItem>
                          </Split>
                        </CardBody>
                      </Card>
                    </GridItem>
                  ))}
                </Grid>
              </CardBody>
            </Card>
          </GridItem>

          {/* External Links Card */}
          <GridItem span={12}>
            <Card>
              <CardTitle>External Links</CardTitle>
              <CardBody>
                <Split hasGutter>
                  <SplitItem>
                    <Button
                      variant="link"
                      icon={<ExternalLinkAltIcon />}
                      component="a"
                      href={project.jiraUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Open JIRA Project
                    </Button>
                  </SplitItem>
                  <SplitItem>
                    <Button
                      variant="link"
                      icon={<ExternalLinkAltIcon />}
                      component="a"
                      href={project.gitRepository}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Open Git Repository
                    </Button>
                  </SplitItem>
                </Split>
              </CardBody>
            </Card>
          </GridItem>
        </Grid>
      </PageSection>
    </Page>
  );
};

export default ProjectDashboard;