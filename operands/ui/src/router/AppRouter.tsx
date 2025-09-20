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

import React, { useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, useParams, useLocation } from 'react-router-dom';
import {
  Page,
  PageHeader,
  PageSidebar,
  PageSection,
  Nav,
  NavList,
  NavItem,
  NavExpandable,
  Brand,
  Avatar,
  Dropdown,
  DropdownItem,
  DropdownSeparator,
  KebabToggle,
  Toolbar,
  ToolbarContent,
  ToolbarGroup,
  ToolbarItem,
  Button,
  Alert,
  AlertVariant,
  Text,
  TextVariants,
  Breadcrumb,
  BreadcrumbItem,
  PageHeaderTools,
  PageHeaderToolsGroup,
  PageHeaderToolsItem
} from '@patternfly/react-core';
import {
  DashboardIcon,
  TaskIcon,
  BugIcon,
  MonitoringIcon,
  SettingsIcon,
  HelpIcon,
  ExternalLinkAltIcon,
  BellIcon,
  CogIcon
} from '@patternfly/react-icons';

// Import components
import ProjectDashboard from '@/components/ProjectDashboard';
import TaskMonitor from '@/components/TaskMonitor';
import IssueBrowser from '@/components/IssueBrowser';
import HealthStatus from '@/components/HealthStatus';

// Route components
const DashboardRoute: React.FC = () => {
  const { projectKey } = useParams<{ projectKey: string }>();
  
  if (!projectKey) {
    return (
      <PageSection>
        <Alert variant={AlertVariant.warning} title="Project Not Found">
          No project key specified in the URL.
        </Alert>
      </PageSection>
    );
  }

  return <ProjectDashboard projectKey={projectKey} />;
};

const TasksRoute: React.FC = () => {
  const { projectKey } = useParams<{ projectKey?: string }>();
  return <TaskMonitor projectKey={projectKey} />;
};

const IssuesRoute: React.FC = () => {
  const { projectKey } = useParams<{ projectKey: string }>();
  
  if (!projectKey) {
    return (
      <PageSection>
        <Alert variant={AlertVariant.warning} title="Project Not Found">
          No project key specified in the URL.
        </Alert>
      </PageSection>
    );
  }

  return <IssueBrowser projectKey={projectKey} />;
};

const HealthRoute: React.FC = () => {
  return (
    <PageSection>
      <HealthStatus />
    </PageSection>
  );
};

const NotFoundRoute: React.FC = () => {
  return (
    <PageSection>
      <Alert variant={AlertVariant.danger} title="Page Not Found">
        The requested page could not be found.
      </Alert>
    </PageSection>
  );
};

const AppNavigation: React.FC<{ 
  activeItem: string;
  onSelect: (selectedItem: string) => void;
  projectKey?: string;
}> = ({ activeItem, onSelect, projectKey }) => {
  const [expandedSections, setExpandedSections] = useState<string[]>(['project']);

  const onToggle = (groupId: string, isExpanded: boolean) => {
    setExpandedSections(prev => 
      isExpanded 
        ? [...prev, groupId]
        : prev.filter(id => id !== groupId)
    );
  };

  return (
    <Nav aria-label="JIRA CDC Navigation" theme="dark">
      <NavList>
        {/* Global Navigation */}
        <NavItem 
          itemId="health" 
          isActive={activeItem === 'health'}
          to="/health"
        >
          <MonitoringIcon /> System Health
        </NavItem>
        
        <NavItem 
          itemId="all-tasks" 
          isActive={activeItem === 'all-tasks'}
          to="/tasks"
        >
          <TaskIcon /> All Tasks
        </NavItem>

        {/* Project-specific Navigation */}
        {projectKey && (
          <NavExpandable
            title="Project"
            groupId="project"
            isActive={['dashboard', 'issues', 'tasks'].includes(activeItem)}
            isExpanded={expandedSections.includes('project')}
            onExpand={(_, isExpanded) => onToggle('project', isExpanded)}
          >
            <NavItem 
              itemId="dashboard" 
              isActive={activeItem === 'dashboard'}
              to={`/project/${projectKey}`}
            >
              <DashboardIcon /> Dashboard
            </NavItem>
            
            <NavItem 
              itemId="issues" 
              isActive={activeItem === 'issues'}
              to={`/project/${projectKey}/issues`}
            >
              <BugIcon /> Issues
            </NavItem>
            
            <NavItem 
              itemId="tasks" 
              isActive={activeItem === 'tasks'}
              to={`/project/${projectKey}/tasks`}
            >
              <TaskIcon /> Tasks
            </NavItem>
          </NavExpandable>
        )}

        {/* Settings and Help */}
        <NavItem 
          itemId="settings" 
          isActive={activeItem === 'settings'}
          to="/settings"
        >
          <SettingsIcon /> Settings
        </NavItem>
        
        <NavItem 
          itemId="help" 
          isActive={activeItem === 'help'}
          href="https://docs.example.com/jiracdc"
          target="_blank"
          rel="noopener noreferrer"
        >
          <HelpIcon /> Documentation <ExternalLinkAltIcon />
        </NavItem>
      </NavList>
    </Nav>
  );
};

const AppHeader: React.FC<{ projectKey?: string }> = ({ projectKey }) => {
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [isNotificationDropdownOpen, setIsNotificationDropdownOpen] = useState(false);

  const userDropdownItems = [
    <DropdownItem key="profile">
      <CogIcon /> User Profile
    </DropdownItem>,
    <DropdownItem key="preferences">
      <SettingsIcon /> Preferences
    </DropdownItem>,
    <DropdownSeparator key="separator" />,
    <DropdownItem key="logout">
      Logout
    </DropdownItem>
  ];

  const notificationDropdownItems = [
    <DropdownItem key="no-notifications">
      No new notifications
    </DropdownItem>
  ];

  const headerTools = (
    <PageHeaderTools>
      <PageHeaderToolsGroup>
        <PageHeaderToolsItem>
          <Dropdown
            isPlain
            position="right"
            onSelect={() => setIsNotificationDropdownOpen(false)}
            toggle={
              <Button 
                variant="plain" 
                aria-label="Notifications"
                onClick={() => setIsNotificationDropdownOpen(!isNotificationDropdownOpen)}
              >
                <BellIcon />
              </Button>
            }
            isOpen={isNotificationDropdownOpen}
            dropdownItems={notificationDropdownItems}
          />
        </PageHeaderToolsItem>
      </PageHeaderToolsGroup>
      
      <PageHeaderToolsGroup>
        <PageHeaderToolsItem>
          <Dropdown
            isPlain
            position="right"
            onSelect={() => setIsDropdownOpen(false)}
            toggle={
              <KebabToggle onToggle={setIsDropdownOpen} />
            }
            isOpen={isDropdownOpen}
            dropdownItems={userDropdownItems}
          />
        </PageHeaderToolsItem>
      </PageHeaderToolsGroup>
    </PageHeaderTools>
  );

  return (
    <PageHeader
      logo={
        <Brand 
          src="/logo.svg" 
          alt="JIRA CDC" 
          widths={{ default: '180px' }}
        >
          <Text component={TextVariants.h4} style={{ color: 'white', marginLeft: '10px' }}>
            JIRA CDC
          </Text>
        </Brand>
      }
      headerTools={headerTools}
      showNavToggle
    />
  );
};

const AppBreadcrumb: React.FC<{ projectKey?: string }> = ({ projectKey }) => {
  const location = useLocation();
  const pathSegments = location.pathname.split('/').filter(Boolean);

  const getBreadcrumbItems = () => {
    const items = [
      <BreadcrumbItem key="home" to="/">
        Home
      </BreadcrumbItem>
    ];

    if (pathSegments.includes('project') && projectKey) {
      items.push(
        <BreadcrumbItem key="project" to={`/project/${projectKey}`}>
          {projectKey}
        </BreadcrumbItem>
      );

      if (pathSegments.includes('issues')) {
        items.push(
          <BreadcrumbItem key="issues" isActive>
            Issues
          </BreadcrumbItem>
        );
      } else if (pathSegments.includes('tasks')) {
        items.push(
          <BreadcrumbItem key="tasks" isActive>
            Tasks
          </BreadcrumbItem>
        );
      }
    } else if (pathSegments.includes('health')) {
      items.push(
        <BreadcrumbItem key="health" isActive>
          System Health
        </BreadcrumbItem>
      );
    } else if (pathSegments.includes('tasks')) {
      items.push(
        <BreadcrumbItem key="all-tasks" isActive>
          All Tasks
        </BreadcrumbItem>
      );
    }

    return items;
  };

  return (
    <PageSection variant="light" padding={{ default: 'noPadding' }}>
      <Breadcrumb>
        {getBreadcrumbItems()}
      </Breadcrumb>
    </PageSection>
  );
};

const AppLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const location = useLocation();
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  
  // Extract project key from URL
  const projectKeyMatch = location.pathname.match(/\/project\/([^\/]+)/);
  const projectKey = projectKeyMatch ? projectKeyMatch[1] : undefined;
  
  // Determine active navigation item
  const getActiveItem = () => {
    const path = location.pathname;
    
    if (path === '/health') return 'health';
    if (path === '/tasks') return 'all-tasks';
    if (path === '/settings') return 'settings';
    if (path.includes('/issues')) return 'issues';
    if (path.includes('/tasks')) return 'tasks';
    if (path.includes('/project/')) return 'dashboard';
    
    return '';
  };

  const sidebar = (
    <PageSidebar
      nav={
        <AppNavigation 
          activeItem={getActiveItem()}
          onSelect={() => {}} // Navigation handled by React Router
          projectKey={projectKey}
        />
      }
      isNavOpen={isSidebarOpen}
      theme="dark"
    />
  );

  return (
    <Page
      header={<AppHeader projectKey={projectKey} />}
      sidebar={sidebar}
      onPageResize={() => setIsSidebarOpen(!isSidebarOpen)}
      breadcrumb={<AppBreadcrumb projectKey={projectKey} />}
      isManagedSidebar
    >
      {children}
    </Page>
  );
};

const AppRouter: React.FC = () => {
  return (
    <Router>
      <AppLayout>
        <Routes>
          {/* Default route - redirect to health */}
          <Route path="/" element={<Navigate to="/health" replace />} />
          
          {/* System routes */}
          <Route path="/health" element={<HealthRoute />} />
          <Route path="/tasks" element={<TasksRoute />} />
          
          {/* Project routes */}
          <Route path="/project/:projectKey" element={<DashboardRoute />} />
          <Route path="/project/:projectKey/issues" element={<IssuesRoute />} />
          <Route path="/project/:projectKey/tasks" element={<TasksRoute />} />
          
          {/* Placeholder routes */}
          <Route 
            path="/settings" 
            element={
              <PageSection>
                <Alert variant={AlertVariant.info} title="Settings">
                  Settings page coming soon.
                </Alert>
              </PageSection>
            } 
          />
          
          {/* 404 route */}
          <Route path="*" element={<NotFoundRoute />} />
        </Routes>
      </AppLayout>
    </Router>
  );
};

export default AppRouter;