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
  Card,
  CardBody,
  CardTitle,
  Stack,
  StackItem,
  Grid,
  GridItem,
  Badge,
  Text,
  TextVariants,
  Alert,
  AlertVariant,
  Spinner,
  Split,
  SplitItem,
  Progress,
  ProgressSize,
  Button,
  Flex,
  FlexItem,
  Timestamp,
  TimestampTooltipVariant,
  List,
  ListItem,
  ExpandableSection
} from '@patternfly/react-core';
import {
  CheckCircleIcon,
  ExclamationTriangleIcon,
  TimesCircleIcon,
  InfoCircleIcon,
  RedoIcon,
  ExternalLinkAltIcon
} from '@patternfly/react-icons';
import { apiService, HealthResponse, ComponentHealth } from '@/services/api';

interface HealthStatusProps {
  autoRefresh?: boolean;
  refreshInterval?: number;
  compact?: boolean;
}

const HealthStatus: React.FC<HealthStatusProps> = ({ 
  autoRefresh = true, 
  refreshInterval = 30000,
  compact = false 
}) => {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());
  const [expandedComponents, setExpandedComponents] = useState<string[]>([]);

  const loadHealth = async () => {
    try {
      setError(null);
      const healthData = await apiService.getHealth();
      setHealth(healthData);
      setLastRefresh(new Date());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load health status');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadHealth();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    
    const interval = setInterval(loadHealth, refreshInterval);
    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval]);

  const getStatusIcon = (status: string, size?: 'sm' | 'md' | 'lg') => {
    const iconProps = { size };
    
    switch (status.toLowerCase()) {
      case 'healthy':
        return <CheckCircleIcon color="green" {...iconProps} />;
      case 'degraded':
        return <ExclamationTriangleIcon color="orange" {...iconProps} />;
      case 'unhealthy':
        return <TimesCircleIcon color="red" {...iconProps} />;
      default:
        return <InfoCircleIcon color="grey" {...iconProps} />;
    }
  };

  const getStatusBadge = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      healthy: { color: 'green', text: 'Healthy' },
      degraded: { color: 'orange', text: 'Degraded' },
      unhealthy: { color: 'red', text: 'Unhealthy' }
    };
    
    const config = statusMap[status.toLowerCase()] || { color: 'grey', text: status };
    return <Badge color={config.color as any}>{config.text}</Badge>;
  };

  const getHealthProgress = () => {
    if (!health) return 0;
    
    const total = health.summary.totalComponents;
    const healthy = health.summary.healthyComponents;
    
    return total > 0 ? (healthy / total) * 100 : 0;
  };

  const toggleComponentExpansion = (componentName: string) => {
    setExpandedComponents(prev => 
      prev.includes(componentName) 
        ? prev.filter(name => name !== componentName)
        : [...prev, componentName]
    );
  };

  const renderComponentDetails = (component: ComponentHealth) => (
    <Stack hasGutter>
      <StackItem>
        <Split hasGutter>
          <SplitItem>
            <Text component={TextVariants.small}>
              <strong>Last Checked:</strong>{' '}
              <Timestamp 
                date={new Date(component.lastChecked)} 
                tooltip={TimestampTooltipVariant.default}
              />
            </Text>
          </SplitItem>
          {component.responseTime && (
            <SplitItem>
              <Text component={TextVariants.small}>
                <strong>Response Time:</strong> {component.responseTime}
              </Text>
            </SplitItem>
          )}
        </Split>
      </StackItem>
      
      {component.message && (
        <StackItem>
          <Text component={TextVariants.small}>
            <strong>Message:</strong> {component.message}
          </Text>
        </StackItem>
      )}
      
      {component.details && Object.keys(component.details).length > 0 && (
        <StackItem>
          <Text component={TextVariants.small}>
            <strong>Details:</strong>
          </Text>
          <List isPlain>
            {Object.entries(component.details).map(([key, value]) => (
              <ListItem key={key}>
                <Text component={TextVariants.small}>
                  {key}: {value}
                </Text>
              </ListItem>
            ))}
          </List>
        </StackItem>
      )}
    </Stack>
  );

  const renderCompactView = () => {
    if (!health) return null;

    return (
      <Card isCompact>
        <CardBody>
          <Split hasGutter>
            <SplitItem>
              {getStatusIcon(health.status, 'md')}
            </SplitItem>
            <SplitItem isFilled>
              <Stack>
                <StackItem>
                  <Text component={TextVariants.h6}>
                    System Health: {getStatusBadge(health.status)}
                  </Text>
                </StackItem>
                <StackItem>
                  <Text component={TextVariants.small}>
                    {health.summary.healthyComponents}/{health.summary.totalComponents} components healthy
                  </Text>
                </StackItem>
              </Stack>
            </SplitItem>
            <SplitItem>
              <Button
                variant="plain"
                icon={<RedoIcon />}
                onClick={loadHealth}
                isLoading={loading}
                aria-label="Refresh health status"
              />
            </SplitItem>
          </Split>
        </CardBody>
      </Card>
    );
  };

  const renderFullView = () => {
    if (!health) return null;

    return (
      <Stack hasGutter>
        {/* Overall Health Card */}
        <StackItem>
          <Card>
            <CardTitle>
              <Split hasGutter>
                <SplitItem isFilled>System Health Overview</SplitItem>
                <SplitItem>
                  <Button
                    variant="plain"
                    icon={<RedoIcon />}
                    onClick={loadHealth}
                    isLoading={loading}
                    aria-label="Refresh health status"
                  />
                </SplitItem>
              </Split>
            </CardTitle>
            <CardBody>
              <Grid hasGutter>
                <GridItem lg={6}>
                  <Stack hasGutter>
                    <StackItem>
                      <Split hasGutter>
                        <SplitItem>
                          {getStatusIcon(health.status, 'lg')}
                        </SplitItem>
                        <SplitItem>
                          <Stack>
                            <StackItem>
                              <Text component={TextVariants.h4}>
                                {getStatusBadge(health.status)}
                              </Text>
                            </StackItem>
                            <StackItem>
                              <Text component={TextVariants.small}>
                                Last updated: {' '}
                                <Timestamp 
                                  date={lastRefresh} 
                                  tooltip={TimestampTooltipVariant.default}
                                />
                              </Text>
                            </StackItem>
                          </Stack>
                        </SplitItem>
                      </Split>
                    </StackItem>
                    
                    <StackItem>
                      <Progress
                        value={getHealthProgress()}
                        title="Component Health"
                        size={ProgressSize.lg}
                      />
                      <Text component={TextVariants.small}>
                        {health.summary.healthyComponents} of {health.summary.totalComponents} components healthy
                      </Text>
                    </StackItem>
                  </Stack>
                </GridItem>
                
                <GridItem lg={6}>
                  <Stack hasGutter>
                    <StackItem>
                      <Text component={TextVariants.h6}>System Information</Text>
                    </StackItem>
                    <StackItem>
                      <List isPlain>
                        <ListItem>
                          <Text component={TextVariants.small}>
                            <strong>Version:</strong> {health.version}
                          </Text>
                        </ListItem>
                        <ListItem>
                          <Text component={TextVariants.small}>
                            <strong>Uptime:</strong> {health.uptime}
                          </Text>
                        </ListItem>
                        <ListItem>
                          <Text component={TextVariants.small}>
                            <strong>Timestamp:</strong>{' '}
                            <Timestamp 
                              date={new Date(health.timestamp)} 
                              tooltip={TimestampTooltipVariant.default}
                            />
                          </Text>
                        </ListItem>
                      </List>
                    </StackItem>
                  </Stack>
                </GridItem>
              </Grid>
            </CardBody>
          </Card>
        </StackItem>

        {/* Component Health Cards */}
        <StackItem>
          <Card>
            <CardTitle>Component Health Details</CardTitle>
            <CardBody>
              <Grid hasGutter>
                {health.components.map((component, index) => (
                  <GridItem lg={6} key={index}>
                    <Card isCompact>
                      <CardBody>
                        <ExpandableSection
                          toggleText={
                            <Split hasGutter>
                              <SplitItem>
                                {getStatusIcon(component.status)}
                              </SplitItem>
                              <SplitItem isFilled>
                                <Text component={TextVariants.h6}>
                                  {component.name.charAt(0).toUpperCase() + component.name.slice(1)}
                                </Text>
                              </SplitItem>
                              <SplitItem>
                                {getStatusBadge(component.status)}
                              </SplitItem>
                            </Split>
                          }
                          onToggle={() => toggleComponentExpansion(component.name)}
                          isExpanded={expandedComponents.includes(component.name)}
                        >
                          {renderComponentDetails(component)}
                        </ExpandableSection>
                      </CardBody>
                    </Card>
                  </GridItem>
                ))}
              </Grid>
            </CardBody>
          </Card>
        </StackItem>

        {/* Health Summary Card */}
        <StackItem>
          <Card>
            <CardTitle>Health Summary</CardTitle>
            <CardBody>
              <Grid hasGutter>
                <GridItem md={3}>
                  <Flex direction={{ default: 'column' }} alignItems={{ default: 'alignItemsCenter' }}>
                    <FlexItem>
                      <CheckCircleIcon color="green" size="lg" />
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.h4}>
                        {health.summary.healthyComponents}
                      </Text>
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.small}>
                        Healthy
                      </Text>
                    </FlexItem>
                  </Flex>
                </GridItem>
                
                <GridItem md={3}>
                  <Flex direction={{ default: 'column' }} alignItems={{ default: 'alignItemsCenter' }}>
                    <FlexItem>
                      <ExclamationTriangleIcon color="orange" size="lg" />
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.h4}>
                        {health.summary.degradedComponents}
                      </Text>
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.small}>
                        Degraded
                      </Text>
                    </FlexItem>
                  </Flex>
                </GridItem>
                
                <GridItem md={3}>
                  <Flex direction={{ default: 'column' }} alignItems={{ default: 'alignItemsCenter' }}>
                    <FlexItem>
                      <TimesCircleIcon color="red" size="lg" />
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.h4}>
                        {health.summary.unhealthyComponents}
                      </Text>
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.small}>
                        Unhealthy
                      </Text>
                    </FlexItem>
                  </Flex>
                </GridItem>
                
                <GridItem md={3}>
                  <Flex direction={{ default: 'column' }} alignItems={{ default: 'alignItemsCenter' }}>
                    <FlexItem>
                      <InfoCircleIcon color="blue" size="lg" />
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.h4}>
                        {health.summary.totalComponents}
                      </Text>
                    </FlexItem>
                    <FlexItem>
                      <Text component={TextVariants.small}>
                        Total
                      </Text>
                    </FlexItem>
                  </Flex>
                </GridItem>
              </Grid>
            </CardBody>
          </Card>
        </StackItem>
      </Stack>
    );
  };

  if (loading && !health) {
    return (
      <Card>
        <CardBody>
          <Flex justifyContent={{ default: 'justifyContentCenter' }}>
            <FlexItem>
              <Spinner size="lg" />
              <Text component={TextVariants.p}>Loading health status...</Text>
            </FlexItem>
          </Flex>
        </CardBody>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardBody>
          <Alert variant={AlertVariant.danger} title="Error loading health status">
            {error}
            <Button variant="link" onClick={loadHealth}>
              Try again
            </Button>
          </Alert>
        </CardBody>
      </Card>
    );
  }

  return compact ? renderCompactView() : renderFullView();
};

export default HealthStatus;