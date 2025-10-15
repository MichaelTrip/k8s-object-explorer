// API client for Kubernetes Object Explorer
class KubernetesAPI {
    constructor(baseURL = '/api') {
        this.baseURL = baseURL;
    }

    // Generic request method
    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
            },
        };

        const config = { ...defaultOptions, ...options };

        try {
            const response = await fetch(url, config);
            
            if (!response.ok) {
                const error = await response.json().catch(() => ({}));
                throw new Error(error.error || `HTTP ${response.status}: ${response.statusText}`);
            }

            // Handle empty responses
            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                return await response.json();
            } else {
                return response;
            }
        } catch (error) {
            console.error(`API request failed: ${endpoint}`, error);
            throw error;
        }
    }

    // Health check
    async healthCheck() {
        return this.request('/health');
    }

    // Connection management
    async testConnection(connectionData) {
        return this.request('/connection/test', {
            method: 'POST',
            body: JSON.stringify(connectionData),
        });
    }

    // Namespaces
    async getNamespaces() {
        return this.request('/namespaces');
    }

    // Resources
    async getResources(filters = {}) {
        const params = new URLSearchParams();
        Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                params.append(key, value.toString());
            }
        });

        const query = params.toString();
        const endpoint = query ? `/resources?${query}` : '/resources';
        return this.request(endpoint);
    }

    async getNamespaceResources(namespace, filters = {}) {
        const params = new URLSearchParams();
        Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                params.append(key, value.toString());
            }
        });

        const query = params.toString();
        const endpoint = query 
            ? `/resources/${namespace}?${query}` 
            : `/resources/${namespace}`;
        return this.request(endpoint);
    }

    async getResourceObjects(namespace, resource, filters = {}) {
        const params = new URLSearchParams();
        Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                params.append(key, value.toString());
            }
        });

        const query = params.toString();
        const endpoint = query 
            ? `/resources/${namespace}/${resource}?${query}` 
            : `/resources/${namespace}/${resource}`;
        return this.request(endpoint);
    }

    async getResourceObject(namespace, resource, name) {
        return this.request(`/resources/${namespace}/${resource}/${name}`);
    }

    // Export functionality
    async exportData(format, data) {
        const response = await this.request(`/export/${format}`, {
            method: 'POST',
            body: JSON.stringify(data),
        });

        // Handle file downloads
        if (response instanceof Response) {
            const blob = await response.blob();
            const filename = this.getFilenameFromResponse(response) || `export.${format}`;
            this.downloadBlob(blob, filename);
        }

        return response;
    }

    // Utility methods
    getFilenameFromResponse(response) {
        const contentDisposition = response.headers.get('Content-Disposition');
        if (contentDisposition) {
            const match = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/);
            if (match && match[1]) {
                return match[1].replace(/['"]/g, '');
            }
        }
        return null;
    }

    downloadBlob(blob, filename) {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.style.display = 'none';
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
    }
}

// Error handling utilities
class APIError extends Error {
    constructor(message, code, details) {
        super(message);
        this.name = 'APIError';
        this.code = code;
        this.details = details;
    }
}

// Create global API instance
window.kubernetesAPI = new KubernetesAPI();

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { KubernetesAPI, APIError };
}