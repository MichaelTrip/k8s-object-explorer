// Main application JavaScript
class KubernetesExplorerApp {
    constructor() {
        this.currentView = 'dashboard';
        this.currentNamespace = '';
        this.currentData = {};
        this.filters = {};
        this.autoRefresh = false; // Explicitly disabled
        this.refreshInterval = 30000; // 30 seconds
        this.refreshTimer = null;
        this.filterTimeout = null; // For debouncing filter changes
        this.isLoadingResources = false; // Prevent recursive calls
        this.lastRequestTime = 0; // Track last request time
        this.requestCooldown = 2000; // 2 second cooldown between requests
        
        console.log('Kubernetes Explorer App constructed');
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.loadInitialData();
        this.updateConnectionStatus('connecting');
    }

    setupEventListeners() {
        // Navigation
        document.querySelectorAll('.nav-item').forEach(item => {
            item.addEventListener('click', (e) => {
                const view = e.currentTarget.dataset.view;
                this.switchView(view);
            });
        });

        // Connection management
        const toggleConnectionBtn = document.getElementById('toggleConnectionPanel');
        const testConnectionBtn = document.getElementById('testConnection');
        const cancelConnectionBtn = document.getElementById('cancelConnection');

        if (toggleConnectionBtn) {
            toggleConnectionBtn.addEventListener('click', () => {
                this.toggleConnectionPanel();
            });
        }

        if (testConnectionBtn) {
            testConnectionBtn.addEventListener('click', () => {
                this.testConnection();
            });
        }

        if (cancelConnectionBtn) {
            cancelConnectionBtn.addEventListener('click', () => {
                this.hideConnectionPanel();
            });
        }

        // Dashboard actions
        const refreshDashboardBtn = document.getElementById('refreshDashboard');
        if (refreshDashboardBtn) {
            refreshDashboardBtn.addEventListener('click', () => {
                this.refreshDashboard();
            });
        }

        // Resource management
        const loadResourcesBtn = document.getElementById('loadResources');
        const namespaceSelect = document.getElementById('namespaceSelect');
        
        if (loadResourcesBtn) {
            loadResourcesBtn.addEventListener('click', (e) => {
                e.preventDefault(); // Prevent any default behavior
                e.stopPropagation(); // Stop event bubbling
                
                console.log('Load Resources button clicked');
                
                if (!this.isLoadingResources && this.currentNamespace) {
                    // Disable the button temporarily to prevent double-clicks
                    loadResourcesBtn.disabled = true;
                    
                    this.loadNamespaceResources().finally(() => {
                        // Re-enable button after request completes
                        loadResourcesBtn.disabled = false;
                    });
                } else if (!this.currentNamespace) {
                    this.showNotification('Please select a namespace first', 'warning');
                } else {
                    console.log('Load already in progress, ignoring click');
                }
            });
        }

        if (namespaceSelect) {
            namespaceSelect.addEventListener('change', (e) => {
                const newNamespace = e.target.value;
                console.log('Namespace changed from', this.currentNamespace, 'to', newNamespace);
                
                // Cancel any ongoing operations
                this.isLoadingResources = false;
                if (this.filterTimeout) {
                    clearTimeout(this.filterTimeout);
                    this.filterTimeout = null;
                }
                
                this.currentNamespace = newNamespace;
                
                // Clear existing data when switching namespaces
                this.currentData.resources = [];
                this.populateResourcesTable([]);
            });
        }

        // Filters
        this.setupFilterListeners();

        // Export functionality
        const exportResourcesBtn = document.getElementById('exportResources');
        if (exportResourcesBtn) {
            exportResourcesBtn.addEventListener('click', () => {
                this.showExportModal();
            });
        }

        // Modal management
        this.setupModalListeners();

        // Settings
        this.setupSettingsListeners();

        // Namespace management
        const refreshNamespacesBtn = document.getElementById('refreshNamespaces');
        if (refreshNamespacesBtn) {
            refreshNamespacesBtn.addEventListener('click', () => {
                this.loadNamespaces();
            });
        }
    }

    setupFilterListeners() {
        const resourceSearch = document.getElementById('resourceSearch');
        const showOnlyPopulated = document.getElementById('showOnlyPopulated');
        const apiGroupFilter = document.getElementById('apiGroupFilter');
        const clearFiltersBtn = document.getElementById('clearFilters');

        // Create a single debounced function for all filters
        const debouncedApplyFilters = debounce(() => {
            console.log('Debounced filter application triggered');
            this.applyFilters();
        }, 500);

        if (resourceSearch) {
            resourceSearch.addEventListener('input', (e) => {
                console.log('Search filter changed:', e.target.value);
                debouncedApplyFilters();
            });
        }

        if (showOnlyPopulated) {
            showOnlyPopulated.addEventListener('change', (e) => {
                console.log('Show only populated changed:', e.target.checked);
                debouncedApplyFilters();
            });
        }

        if (apiGroupFilter) {
            apiGroupFilter.addEventListener('change', (e) => {
                console.log('API Group filter changed:', e.target.value);
                debouncedApplyFilters();
            });
        }

        if (clearFiltersBtn) {
            clearFiltersBtn.addEventListener('click', () => {
                console.log('Clear filters clicked');
                this.clearFilters();
            });
        }
    }

    setupModalListeners() {
        const modalBackdrop = document.getElementById('modalBackdrop');
        const closeModalBtns = document.querySelectorAll('.modal-close');

        if (modalBackdrop) {
            modalBackdrop.addEventListener('click', (e) => {
                if (e.target === modalBackdrop) {
                    this.hideModal();
                }
            });
        }

        closeModalBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                this.hideModal();
            });
        });

        // Export modal
        const confirmExportBtn = document.getElementById('confirmExport');
        const cancelExportBtn = document.getElementById('cancelExport');

        if (confirmExportBtn) {
            confirmExportBtn.addEventListener('click', () => {
                this.performExport();
            });
        }

        if (cancelExportBtn) {
            cancelExportBtn.addEventListener('click', () => {
                this.hideModal();
            });
        }
    }

    setupSettingsListeners() {
        const autoRefreshCheckbox = document.getElementById('autoRefresh');
        const refreshIntervalInput = document.getElementById('refreshInterval');

        if (autoRefreshCheckbox) {
            autoRefreshCheckbox.addEventListener('change', (e) => {
                this.autoRefresh = e.target.checked;
                this.updateAutoRefresh();
            });
        }

        if (refreshIntervalInput) {
            refreshIntervalInput.addEventListener('change', (e) => {
                this.refreshInterval = parseInt(e.target.value) * 1000;
                if (this.autoRefresh) {
                    this.updateAutoRefresh();
                }
            });
        }
    }

    async loadInitialData() {
        try {
            console.log('Loading initial data...');
            // Check health status
            const health = await window.kubernetesAPI.healthCheck();
            this.updateHealthStatus(health);

            // Load namespaces if connected
            if (health.status === 'healthy') {
                console.log('Health check passed, loading namespaces');
                await this.loadNamespaces();
                this.updateConnectionStatus('connected');
            } else {
                console.log('Health check failed, not loading namespaces');
            }
        } catch (error) {
            console.error('Failed to load initial data:', error);
            this.updateConnectionStatus('disconnected');
        }
    }

    async loadNamespaces() {
        try {
            this.showLoading();
            const response = await window.kubernetesAPI.getNamespaces();
            this.populateNamespaceSelector(response.namespaces);
            this.populateNamespaceCards(response.namespaces);
            this.updateDashboardStats('totalNamespaces', response.count);
        } catch (error) {
            console.error('Failed to load namespaces:', error);
            this.showNotification('Failed to load namespaces: ' + error.message, 'error');
        } finally {
            this.hideLoading();
        }
    }

    async loadNamespaceResources() {
        if (!this.currentNamespace) {
            this.showNotification('Please select a namespace first', 'warning');
            return;
        }

        // Prevent recursive calls with better logging
        if (this.isLoadingResources) {
            console.warn('Already loading resources for namespace:', this.currentNamespace, '- skipping duplicate request');
            return;
        }

        // Add cooldown period to prevent rapid requests
        const now = Date.now();
        if (now - this.lastRequestTime < this.requestCooldown) {
            console.warn(`Request cooldown active. Last request was ${now - this.lastRequestTime}ms ago. Skipping.`);
            return;
        }

        // Add a request timestamp to track duplicate calls
        const requestId = Date.now();
        console.log(`[${requestId}] Starting resource load for namespace:`, this.currentNamespace);
        
        this.lastRequestTime = now;

        try {
            this.isLoadingResources = true;
            this.showLoading();
            
            const filters = this.getActiveFilters();
            console.log(`[${requestId}] Loading with filters:`, filters);
            
            const response = await window.kubernetesAPI.getNamespaceResources(this.currentNamespace, filters);
            
            // Check if we're still supposed to be loading (user might have changed namespace)
            if (!this.isLoadingResources) {
                console.log(`[${requestId}] Load cancelled - isLoadingResources flag was cleared`);
                return;
            }
            
            this.currentData.resources = response.resources;
            this.populateResourcesTable(response.resources);
            this.populateApiGroupFilter(response.resources);
            this.updateDashboardStats('totalObjects', response.totalObjects);
            
            this.logActivity(`Loaded ${response.count} resources from namespace "${this.currentNamespace}"`);
            console.log(`[${requestId}] Successfully loaded ${response.count} resources`);
        } catch (error) {
            console.error(`[${requestId}] Failed to load namespace resources:`, error);
            this.showNotification('Failed to load resources: ' + error.message, 'error');
        } finally {
            this.isLoadingResources = false;
            this.hideLoading();
            console.log(`[${requestId}] Finished resource load, flag reset`);
        }
    }

    async testConnection() {
        const kubeconfigPath = document.getElementById('kubeconfigPath').value;
        const apiServerUrl = document.getElementById('apiServerUrl').value;

        const connectionData = {
            kubeconfigPath: kubeconfigPath || undefined,
            apiServerUrl: apiServerUrl || undefined
        };

        try {
            this.showLoading();
            const response = await window.kubernetesAPI.testConnection(connectionData);
            
            this.showNotification('Successfully connected to Kubernetes cluster', 'success');
            this.updateConnectionStatus('connected');
            this.hideConnectionPanel();
            
            // Reload data with new connection
            await this.loadNamespaces();
        } catch (error) {
            console.error('Connection test failed:', error);
            this.showNotification('Connection failed: ' + error.message, 'error');
            this.updateConnectionStatus('disconnected');
        } finally {
            this.hideLoading();
        }
    }

    switchView(viewName) {
        console.log('Switching view from', this.currentView, 'to', viewName);
        
        // Update navigation
        document.querySelectorAll('.nav-item').forEach(item => {
            item.classList.toggle('active', item.dataset.view === viewName);
        });

        // Update views
        document.querySelectorAll('.view').forEach(view => {
            view.classList.toggle('active', view.id === `${viewName}View`);
        });

        this.currentView = viewName;

        // Load view-specific data
        if (viewName === 'namespaces') {
            console.log('Loading namespaces for namespaces view');
            this.loadNamespaces();
        }
    }

    getActiveFilters() {
        const resourceSearch = document.getElementById('resourceSearch');
        const showOnlyPopulated = document.getElementById('showOnlyPopulated');
        const apiGroupFilter = document.getElementById('apiGroupFilter');

        return {
            search: resourceSearch ? resourceSearch.value : '',
            populated: showOnlyPopulated ? showOnlyPopulated.checked : false,
            apiGroup: apiGroupFilter ? apiGroupFilter.value : ''
        };
    }

    applyFilters() {
        // Clear any pending filter requests to prevent rapid-fire calls
        if (this.filterTimeout) {
            clearTimeout(this.filterTimeout);
        }
        
        // Only apply filters if we have a namespace and aren't already loading
        if (this.currentNamespace && !this.isLoadingResources) {
            console.log('Applying filters for namespace:', this.currentNamespace);
            this.loadNamespaceResources();
        } else {
            console.log('Skipping filter application - namespace:', this.currentNamespace, 'isLoading:', this.isLoadingResources);
        }
    }

    clearFilters() {
        const resourceSearch = document.getElementById('resourceSearch');
        const showOnlyPopulated = document.getElementById('showOnlyPopulated');
        const apiGroupFilter = document.getElementById('apiGroupFilter');

        console.log('Clearing all filters');
        
        if (resourceSearch) resourceSearch.value = '';
        if (showOnlyPopulated) showOnlyPopulated.checked = false;
        if (apiGroupFilter) apiGroupFilter.value = '';

        // Clear any pending filter timeouts
        if (this.filterTimeout) {
            clearTimeout(this.filterTimeout);
            this.filterTimeout = null;
        }

        // Apply filters with a small delay to ensure UI updates are processed
        setTimeout(() => {
            if (!this.isLoadingResources) {
                this.applyFilters();
            }
        }, 100);
    }

    populateNamespaceSelector(namespaces) {
        const select = document.getElementById('namespaceSelect');
        if (!select) return;

        // Clear existing options except the first
        while (select.children.length > 1) {
            select.removeChild(select.lastChild);
        }

        namespaces.forEach(namespace => {
            const option = document.createElement('option');
            option.value = namespace;
            option.textContent = namespace;
            select.appendChild(option);
        });
    }

    populateNamespaceCards(namespaces) {
        const container = document.getElementById('namespacesGrid');
        if (!container) return;

        container.innerHTML = '';

        namespaces.forEach(namespace => {
            const card = this.createNamespaceCard(namespace);
            container.appendChild(card);
        });
    }

    createNamespaceCard(namespace) {
        const card = document.createElement('div');
        card.className = 'namespace-card';
        card.innerHTML = `
            <div class="namespace-name">${namespace}</div>
            <div class="namespace-stats">
                <div>Resources loading...</div>
            </div>
        `;

        card.addEventListener('click', () => {
            document.getElementById('namespaceSelect').value = namespace;
            this.currentNamespace = namespace;
            this.switchView('resources');
            this.loadNamespaceResources();
        });

        return card;
    }

    populateResourcesTable(resources) {
        const tbody = document.getElementById('resourcesTableBody');
        if (!tbody) return;

        if (resources.length === 0) {
            tbody.innerHTML = `
                <tr class="empty-state">
                    <td colspan="6">
                        <div class="empty-message">No resources found with current filters</div>
                    </td>
                </tr>
            `;
            return;
        }

        tbody.innerHTML = '';

        resources.forEach(resource => {
            const row = this.createResourceRow(resource);
            tbody.appendChild(row);
        });
    }

    createResourceRow(resource) {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${resource.name}</td>
            <td>${resource.kind}</td>
            <td>${resource.shortName || '-'}</td>
            <td>${resource.apiGroup || 'core'}</td>
            <td>
                <span class="resource-count ${resource.count > 0 ? 'populated' : 'empty'}">
                    ${resource.count}
                </span>
            </td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="app.viewResourceObjects('${resource.name}')">
                    View Objects
                </button>
            </td>
        `;

        return row;
    }

    async viewResourceObjects(resourceName) {
        if (!this.currentNamespace) return;

        try {
            this.showLoading();
            const response = await window.kubernetesAPI.getResourceObjects(this.currentNamespace, resourceName);
            this.showResourceObjectsModal(resourceName, response.objects);
        } catch (error) {
            console.error('Failed to load resource objects:', error);
            this.showNotification('Failed to load objects: ' + error.message, 'error');
        } finally {
            this.hideLoading();
        }
    }

    showResourceObjectsModal(resourceName, objects) {
        const modal = document.getElementById('resourceDetailsModal');
        const title = document.getElementById('modalTitle');
        const content = document.getElementById('modalContent');

        title.textContent = `${resourceName} Objects`;
        
        if (objects.length === 0) {
            content.innerHTML = '<p>No objects found for this resource type.</p>';
        } else {
            content.innerHTML = `
                <div class="objects-list">
                    ${objects.map(obj => `
                        <div class="object-item">
                            <strong>${obj.name}</strong>
                            <div class="object-meta">
                                Created: ${new Date(obj.creationTimestamp).toLocaleString()}
                            </div>
                        </div>
                    `).join('')}
                </div>
            `;
        }

        this.showModal();
    }

    populateApiGroupFilter(resources) {
        const select = document.getElementById('apiGroupFilter');
        if (!select) return;

        // Temporarily disable event listener to prevent loops
        const currentValue = select.value;
        
        // Get unique API groups
        const apiGroups = [...new Set(resources.map(r => r.apiGroup || 'core'))].sort();

        // Clear existing options except the first
        while (select.children.length > 1) {
            select.removeChild(select.lastChild);
        }

        apiGroups.forEach(group => {
            const option = document.createElement('option');
            option.value = group === 'core' ? '' : group;
            option.textContent = group;
            select.appendChild(option);
        });

        // Restore the previous value if it still exists
        if (currentValue && [...select.options].some(opt => opt.value === currentValue)) {
            select.value = currentValue;
        }
    }

    updateConnectionStatus(status) {
        const statusIndicator = document.getElementById('statusIndicator');
        const statusText = document.getElementById('statusText');

        if (!statusIndicator || !statusText) return;

        statusIndicator.className = 'status-indicator ' + status;
        
        const statusMessages = {
            connected: 'Connected',
            disconnected: 'Disconnected',
            connecting: 'Connecting...',
            error: 'Connection Error'
        };

        statusText.textContent = statusMessages[status] || 'Unknown';
    }

    updateHealthStatus(health) {
        // Update CLI status indicators
        const kubectlStatus = document.getElementById('kubectlStatus');
        const ocStatus = document.getElementById('ocStatus');

        if (kubectlStatus) {
            kubectlStatus.textContent = health.features.kubectlAvailable ? 'Available' : 'Not Available';
            kubectlStatus.className = `cli-status-indicator ${health.features.kubectlAvailable ? 'available' : 'unavailable'}`;
        }

        if (ocStatus) {
            ocStatus.textContent = health.features.ocAvailable ? 'Available' : 'Not Available';
            ocStatus.className = `cli-status-indicator ${health.features.ocAvailable ? 'available' : 'unavailable'}`;
        }
    }

    updateDashboardStats(stat, value) {
        const element = document.getElementById(stat);
        if (element) {
            element.textContent = value.toLocaleString();
        }
    }

    refreshDashboard() {
        this.loadInitialData();
    }

    updateAutoRefresh() {
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
            this.refreshTimer = null;
        }

        if (this.autoRefresh) {
            console.log('Setting up auto-refresh with interval:', this.refreshInterval);
            this.refreshTimer = setInterval(() => {
                // Only refresh if not currently loading to prevent conflicts
                if (this.currentView === 'resources' && this.currentNamespace && !this.isLoadingResources) {
                    console.log('Auto-refresh triggered for resources view');
                    this.loadNamespaceResources();
                } else if (this.currentView === 'dashboard') {
                    console.log('Auto-refresh triggered for dashboard view');
                    this.refreshDashboard();
                } else {
                    console.log('Auto-refresh skipped - conditions not met');
                }
            }, this.refreshInterval);
        }
    }

    logActivity(message) {
        const log = document.getElementById('activityLog');
        if (!log) return;

        const item = document.createElement('div');
        item.className = 'activity-item';
        item.innerHTML = `
            <span class="activity-time">${new Date().toLocaleTimeString()}</span>
            <span class="activity-message">${message}</span>
        `;

        log.insertBefore(item, log.firstChild);

        // Keep only the last 20 items
        while (log.children.length > 20) {
            log.removeChild(log.lastChild);
        }
    }

    // UI utility methods
    showLoading() {
        const overlay = document.getElementById('loadingOverlay');
        if (overlay) {
            overlay.style.display = 'flex';
        }
    }

    hideLoading() {
        const overlay = document.getElementById('loadingOverlay');
        if (overlay) {
            overlay.style.display = 'none';
        }
    }

    showNotification(message, type = 'info') {
        const container = document.getElementById('notificationsContainer');
        if (!container) return;

        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.textContent = message;

        container.appendChild(notification);

        // Auto-remove after 5 seconds
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 5000);
    }

    showModal() {
        const backdrop = document.getElementById('modalBackdrop');
        if (backdrop) {
            backdrop.style.display = 'flex';
        }
    }

    hideModal() {
        const backdrop = document.getElementById('modalBackdrop');
        if (backdrop) {
            backdrop.style.display = 'none';
        }
    }

    toggleConnectionPanel() {
        const form = document.getElementById('connectionForm');
        if (form) {
            form.style.display = form.style.display === 'none' ? 'block' : 'none';
        }
    }

    hideConnectionPanel() {
        const form = document.getElementById('connectionForm');
        if (form) {
            form.style.display = 'none';
        }
    }

    showExportModal() {
        const exportModal = document.getElementById('exportModal');
        const backdrop = document.getElementById('modalBackdrop');
        
        if (exportModal && backdrop) {
            // Hide other modals
            document.querySelectorAll('.modal').forEach(modal => {
                modal.style.display = 'none';
            });
            
            exportModal.style.display = 'block';
            backdrop.style.display = 'flex';
        }
    }

    async performExport() {
        const formatRadios = document.querySelectorAll('input[name="exportFormat"]');
        let format = 'json';

        formatRadios.forEach(radio => {
            if (radio.checked) {
                format = radio.value;
            }
        });

        try {
            this.showLoading();
            await window.kubernetesAPI.exportData(format, this.currentData);
            this.showNotification(`Data exported as ${format.toUpperCase()}`, 'success');
            this.hideModal();
        } catch (error) {
            console.error('Export failed:', error);
            this.showNotification('Export failed: ' + error.message, 'error');
        } finally {
            this.hideLoading();
        }
    }
}

// Utility functions
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Initialize app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    // Prevent multiple app instances
    if (window.app) {
        console.warn('App already initialized, skipping duplicate initialization');
        return;
    }
    
    console.log('Initializing Kubernetes Explorer App');
    window.app = new KubernetesExplorerApp();
});

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = KubernetesExplorerApp;
}