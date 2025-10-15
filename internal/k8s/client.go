package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client represents a Kubernetes client with discovery capabilities
type Client struct {
	clientset       *kubernetes.Clientset
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	config          *rest.Config

	// Cache for resource discovery
	resourcesCache     []ResourceInfo
	resourcesCacheTime time.Time
	cacheTTL           time.Duration

	// Cache for namespace resource counts
	namespaceCaches     map[string][]ResourceInfo // namespace -> resources with counts
	namespaceCacheTimes map[string]time.Time      // namespace -> cache time
}

// ResourceInfo contains information about a Kubernetes resource
type ResourceInfo struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`    // name.apiGroup for uniqueness
	DisplayName string `json:"displayName"` // name (apiGroup) for display
	Kind        string `json:"kind"`
	ShortName   string `json:"shortName,omitempty"`
	APIGroup    string `json:"apiGroup"`
	APIVersion  string `json:"apiVersion"`
	Namespaced  bool   `json:"namespaced"`
	Count       int    `json:"count"`
}

// ObjectInfo contains information about a Kubernetes object
type ObjectInfo struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace,omitempty"`
	Kind              string                 `json:"kind"`
	APIVersion        string                 `json:"apiVersion"`
	CreationTimestamp time.Time              `json:"creationTimestamp"`
	Labels            map[string]string      `json:"labels,omitempty"`
	Annotations       map[string]string      `json:"annotations,omitempty"`
	Status            map[string]interface{} `json:"status,omitempty"`
	Spec              map[string]interface{} `json:"spec,omitempty"`
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// Try to use kubeconfig file first
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// Fall back to in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %v", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	return &Client{
		clientset:           clientset,
		dynamicClient:       dynamicClient,
		discoveryClient:     discoveryClient,
		config:              config,
		cacheTTL:            5 * time.Minute, // Cache for 5 minutes
		namespaceCaches:     make(map[string][]ResourceInfo),
		namespaceCacheTimes: make(map[string]time.Time),
	}, nil
}

// GetNamespaces returns a list of all namespaces
func (c *Client) GetNamespaces() ([]string, error) {
	if c.clientset == nil {
		return nil, fmt.Errorf("no kubernetes client available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, len(namespaces.Items))
	for i, ns := range namespaces.Items {
		result[i] = ns.Name
	}

	return result, nil
}

// GetAPIResources returns all available API resources with caching
func (c *Client) GetAPIResources() ([]ResourceInfo, error) {
	if c.discoveryClient == nil {
		return nil, fmt.Errorf("no discovery client available")
	}

	// Check cache first
	if len(c.resourcesCache) > 0 && time.Since(c.resourcesCacheTime) < c.cacheTTL {
		log.Printf("[DEBUG] Using cached API resources (%d resources, cached %v ago)",
			len(c.resourcesCache), time.Since(c.resourcesCacheTime).Round(time.Second))
		return c.resourcesCache, nil
	}

	log.Printf("[DEBUG] Cache miss or expired, discovering API resources...")
	start := time.Now()

	// Use ServerPreferredNamespacedResources
	resourceLists, err := c.discoveryClient.ServerPreferredNamespacedResources()
	if err != nil {
		// Handle partial discovery errors - many clusters have some APIs that fail
		if discovery.IsGroupDiscoveryFailedError(err) {
			groupErr := err.(*discovery.ErrGroupDiscoveryFailed)
			log.Printf("Warning: Some API groups failed discovery: %v", len(groupErr.Groups))
			// Continue with whatever we successfully discovered
			if len(resourceLists) == 0 {
				// Return a minimal set of core resources that should always be available
				coreResources := []ResourceInfo{
					{Name: "pods", FullName: "pods", DisplayName: "pods", Kind: "Pod", ShortName: "po", APIGroup: "", APIVersion: "v1", Namespaced: true},
					{Name: "services", FullName: "services", DisplayName: "services", Kind: "Service", ShortName: "svc", APIGroup: "", APIVersion: "v1", Namespaced: true},
					{Name: "configmaps", FullName: "configmaps", DisplayName: "configmaps", Kind: "ConfigMap", ShortName: "cm", APIGroup: "", APIVersion: "v1", Namespaced: true},
					{Name: "secrets", FullName: "secrets", DisplayName: "secrets", Kind: "Secret", ShortName: "", APIGroup: "", APIVersion: "v1", Namespaced: true},
					{Name: "deployments", FullName: "deployments.apps", DisplayName: "deployments (apps)", Kind: "Deployment", ShortName: "deploy", APIGroup: "apps", APIVersion: "v1", Namespaced: true},
				}
				log.Printf("Using core resources fallback: %d resources", len(coreResources))
				return coreResources, nil
			}
		} else {
			return nil, fmt.Errorf("failed to discover API resources: %v", err)
		}
	}

	var resources []ResourceInfo
	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, resource := range resourceList.APIResources {
			// Skip subresources
			if strings.Contains(resource.Name, "/") {
				continue
			}

			shortName := ""
			if len(resource.ShortNames) > 0 {
				shortName = resource.ShortNames[0]
			}

			// Create unique identifier and display name
			fullName := resource.Name
			displayName := resource.Name
			if gv.Group != "" {
				fullName = resource.Name + "." + gv.Group
				displayName = resource.Name + " (" + gv.Group + ")"
			}

			resources = append(resources, ResourceInfo{
				Name:        resource.Name,
				FullName:    fullName,
				DisplayName: displayName,
				Kind:        resource.Kind,
				ShortName:   shortName,
				APIGroup:    gv.Group,
				APIVersion:  gv.Version,
				Namespaced:  resource.Namespaced,
			})
		}
	}

	// Update cache
	c.resourcesCache = resources
	c.resourcesCacheTime = time.Now()
	log.Printf("[DEBUG] API resource discovery completed in %v, cached %d resources",
		time.Since(start), len(resources))

	return resources, nil
}

// GetResourcesInNamespace returns resources with object counts for a specific namespace with caching
func (c *Client) GetResourcesInNamespace(namespace string) ([]ResourceInfo, error) {
	// Check namespace cache first
	if cachedResources, exists := c.namespaceCaches[namespace]; exists {
		if cacheTime, timeExists := c.namespaceCacheTimes[namespace]; timeExists {
			if time.Since(cacheTime) < c.cacheTTL {
				log.Printf("[DEBUG] Using cached namespace data for '%s' (%d resources, cached %v ago)",
					namespace, len(cachedResources), time.Since(cacheTime).Round(time.Second))
				return cachedResources, nil
			} else {
				log.Printf("[DEBUG] Cache expired for namespace '%s', refreshing...", namespace)
			}
		}
	} else {
		log.Printf("[DEBUG] No cache found for namespace '%s', counting objects...", namespace)
	}
	resources, err := c.GetAPIResources()
	if err != nil {
		return nil, err
	}

	// Filter to only namespaced resources and skip problematic ones
	var namespacedResources []ResourceInfo
	skipResources := map[string]bool{
		"bindings":                  true,
		"localsubjectaccessreviews": true,
		"selfsubjectaccessreviews":  true,
		"selfsubjectrulesreviews":   true,
		"uploadtokenrequests":       true,
		"tokenrequests":             true,
		"subjectaccessreviews":      true,
	}

	for _, resource := range resources {
		if resource.Namespaced && !skipResources[resource.Name] {
			namespacedResources = append(namespacedResources, resource)
		}
	}

	log.Printf("Counting objects for %d namespaced resources in namespace '%s'", len(namespacedResources), namespace)

	// Count objects with progress reporting
	processed := 0
	debugMode := os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1"

	for i := range namespacedResources {
		processed++
		resource := &namespacedResources[i]

		if debugMode && processed <= 10 {
			// Show first 10 resources in detail when debug is enabled
			log.Printf("[DEBUG] Counting objects for: %s (%s/%s)",
				resource.DisplayName, resource.APIGroup, resource.APIVersion)
		}

		count, err := c.countResourceObjects(namespace, *resource)
		if err != nil {
			// Skip common permission errors without logging
			if strings.Contains(err.Error(), "does not allow this method") ||
				strings.Contains(err.Error(), "forbidden") {
				resource.Count = 0
				if debugMode && processed <= 10 {
					log.Printf("[DEBUG]   → Permission denied (expected)")
				}
			} else {
				log.Printf("Warning: Failed to count objects for resource %s: %v", resource.Name, err)
				resource.Count = 0
			}
		} else {
			resource.Count = count
			if debugMode && (count > 0 || processed <= 10) {
				log.Printf("[DEBUG]   → %d objects found", count)
			}
		}

		// Log progress every 20 resources to reduce noise
		if processed%20 == 0 {
			log.Printf("Processed %d/%d resources", processed, len(namespacedResources))
		}
	}

	log.Printf("Completed: Found %d namespaced resources in '%s'", len(namespacedResources), namespace)

	// Cache the results
	c.namespaceCaches[namespace] = namespacedResources
	c.namespaceCacheTimes[namespace] = time.Now()
	log.Printf("[DEBUG] Cached %d resources for namespace '%s'", len(namespacedResources), namespace)

	return namespacedResources, nil
}

// GetResourcesInNamespaceWithCallback returns resources with real-time debug callbacks
func (c *Client) GetResourcesInNamespaceWithCallback(namespace string, debugCallback func(string)) ([]ResourceInfo, error) {
	// Check namespace cache first
	if cachedResources, exists := c.namespaceCaches[namespace]; exists {
		if cacheTime, timeExists := c.namespaceCacheTimes[namespace]; timeExists {
			if time.Since(cacheTime) < c.cacheTTL {
				if debugCallback != nil {
					debugCallback(fmt.Sprintf("⚡ Using cached data for '%s' (%d resources, cached %v ago)",
						namespace, len(cachedResources), time.Since(cacheTime).Round(time.Second)))
				}
				return cachedResources, nil
			}
		}
	}

	if debugCallback != nil {
		debugCallback(fmt.Sprintf("🔍 No cache found for namespace '%s', discovering resources...", namespace))
	}

	log.Printf("[DEBUG] No cache found for namespace '%s', counting objects...", namespace)

	// Get API resources (cached)
	resources, err := c.GetAPIResources()
	if err != nil {
		return nil, err
	}

	if debugCallback != nil {
		debugCallback(fmt.Sprintf("📋 Found %d API resource types, filtering for namespace '%s'", len(resources), namespace))
	}

	// Filter namespaced resources
	var namespacedResources []ResourceInfo
	for _, resource := range resources {
		if resource.Namespaced {
			namespacedResources = append(namespacedResources, resource)
		}
	}

	if debugCallback != nil {
		debugCallback(fmt.Sprintf("🔢 Counting objects for %d namespaced resources in namespace '%s'", len(namespacedResources), namespace))
	}
	log.Printf("Counting objects for %d namespaced resources in namespace '%s'", len(namespacedResources), namespace)

	// Count objects for each resource with real-time updates
	debugMode := strings.ToLower(os.Getenv("DEBUG")) == "true"
	processed := 0

	for i := range namespacedResources {
		resource := &namespacedResources[i]
		processed++

		if debugCallback != nil && debugMode && processed <= 15 {
			debugCallback(fmt.Sprintf("🔍 Counting objects for: %s (%s/%s)",
				resource.DisplayName, resource.APIGroup, resource.APIVersion))
		}

		if debugMode && processed <= 10 {
			log.Printf("[DEBUG] Counting objects for: %s (%s/%s)",
				resource.DisplayName, resource.APIGroup, resource.APIVersion)
		}

		count, err := c.countResourceObjects(namespace, *resource)
		if err != nil {
			if strings.Contains(err.Error(), "does not allow this method") ||
				strings.Contains(err.Error(), "forbidden") {
				resource.Count = 0
				if debugCallback != nil && debugMode && processed <= 15 {
					debugCallback(fmt.Sprintf("  ⚠️ Permission denied (expected)"))
				}
			} else {
				log.Printf("Warning: Failed to count objects for resource %s: %v", resource.Name, err)
				resource.Count = 0
			}
		} else {
			resource.Count = count
			if debugCallback != nil && count > 0 {
				debugCallback(fmt.Sprintf("  ✅ %s: %d objects found", resource.DisplayName, count))
			}
			if debugMode && (count > 0 || processed <= 10) {
				log.Printf("[DEBUG]   → %d objects found", count)
			}
		}

		// Send progress updates via callback
		if debugCallback != nil && (processed%10 == 0 || processed == len(namespacedResources)) {
			progress := int((float64(processed) / float64(len(namespacedResources))) * 100)
			debugCallback(fmt.Sprintf("📈 Progress: %d%% (%d/%d resources)", progress, processed, len(namespacedResources)))
		}

		// Log progress every 20 resources to reduce noise
		if processed%20 == 0 {
			log.Printf("Processed %d/%d resources", processed, len(namespacedResources))
		}
	}

	if debugCallback != nil {
		debugCallback(fmt.Sprintf("✨ Resource discovery complete! Found %d namespaced resources", len(namespacedResources)))
	}

	log.Printf("Completed: Found %d namespaced resources in '%s'", len(namespacedResources), namespace)

	// Cache the results
	c.namespaceCaches[namespace] = namespacedResources
	c.namespaceCacheTimes[namespace] = time.Now()
	log.Printf("[DEBUG] Cached %d resources for namespace '%s'", len(namespacedResources), namespace)

	return namespacedResources, nil
}

// GetResourceObjects returns all objects of a specific resource type in a namespace
func (c *Client) GetResourceObjects(namespace, resourceIdentifier string) ([]ObjectInfo, error) {
	if c.dynamicClient == nil {
		return nil, fmt.Errorf("no dynamic client available")
	}

	// Find the resource info
	resources, err := c.GetAPIResources()
	if err != nil {
		return nil, err
	}

	var targetResource *ResourceInfo
	for _, resource := range resources {
		// Try to match by FullName first (exact match including API group)
		// Then fall back to Name match for backward compatibility
		if (resource.FullName == resourceIdentifier || resource.Name == resourceIdentifier) && resource.Namespaced {
			targetResource = &resource
			break
		}
	}

	if targetResource == nil {
		return nil, fmt.Errorf("resource %s not found or not namespaced", resourceIdentifier)
	}

	gvr := schema.GroupVersionResource{
		Group:    targetResource.APIGroup,
		Version:  targetResource.APIVersion,
		Resource: targetResource.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	objects := make([]ObjectInfo, len(list.Items))
	for i, item := range list.Items {
		objects[i] = ObjectInfo{
			Name:              item.GetName(),
			Namespace:         item.GetNamespace(),
			Kind:              item.GetKind(),
			APIVersion:        item.GetAPIVersion(),
			CreationTimestamp: item.GetCreationTimestamp().Time,
			Labels:            item.GetLabels(),
			Annotations:       item.GetAnnotations(),
		}

		// Extract status and spec if available
		if status, found := item.Object["status"].(map[string]interface{}); found {
			objects[i].Status = status
		}
		if spec, found := item.Object["spec"].(map[string]interface{}); found {
			objects[i].Spec = spec
		}
	}

	return objects, nil
}

// GetResourceObject returns a specific object
func (c *Client) GetResourceObject(namespace, resourceIdentifier, objectName string) (*ObjectInfo, error) {
	if c.dynamicClient == nil {
		return nil, fmt.Errorf("no dynamic client available")
	}

	// Find the resource info
	resources, err := c.GetAPIResources()
	if err != nil {
		return nil, err
	}

	var targetResource *ResourceInfo
	for _, resource := range resources {
		// Try to match by FullName first (exact match including API group)
		// Then fall back to Name match for backward compatibility
		if (resource.FullName == resourceIdentifier || resource.Name == resourceIdentifier) && resource.Namespaced {
			targetResource = &resource
			break
		}
	}

	if targetResource == nil {
		return nil, fmt.Errorf("resource %s not found or not namespaced", resourceIdentifier)
	}

	gvr := schema.GroupVersionResource{
		Group:    targetResource.APIGroup,
		Version:  targetResource.APIVersion,
		Resource: targetResource.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	item, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, objectName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	object := &ObjectInfo{
		Name:              item.GetName(),
		Namespace:         item.GetNamespace(),
		Kind:              item.GetKind(),
		APIVersion:        item.GetAPIVersion(),
		CreationTimestamp: item.GetCreationTimestamp().Time,
		Labels:            item.GetLabels(),
		Annotations:       item.GetAnnotations(),
	}

	// Extract status and spec if available
	if status, found := item.Object["status"].(map[string]interface{}); found {
		object.Status = status
	}
	if spec, found := item.Object["spec"].(map[string]interface{}); found {
		object.Spec = spec
	}

	return object, nil
}

// GetRawResourceObject returns the complete raw Kubernetes object for YAML display
func (c *Client) GetRawResourceObject(namespace, resourceIdentifier, objectName string) (map[string]interface{}, error) {
	if c.dynamicClient == nil {
		return nil, fmt.Errorf("no dynamic client available")
	}

	// Find the resource info
	resources, err := c.GetAPIResources()
	if err != nil {
		return nil, err
	}

	var targetResource *ResourceInfo
	for _, resource := range resources {
		// Try to match by FullName first (exact match including API group)
		// Then fall back to Name match for backward compatibility
		if (resource.FullName == resourceIdentifier || resource.Name == resourceIdentifier) && resource.Namespaced {
			targetResource = &resource
			break
		}
	}

	if targetResource == nil {
		return nil, fmt.Errorf("resource %s not found or not namespaced", resourceIdentifier)
	}

	gvr := schema.GroupVersionResource{
		Group:    targetResource.APIGroup,
		Version:  targetResource.APIVersion,
		Resource: targetResource.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	item, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, objectName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Return the complete raw object for proper YAML conversion
	return item.Object, nil
}

// countResourceObjects counts the number of objects for a resource in a namespace
func (c *Client) countResourceObjects(namespace string, resource ResourceInfo) (int, error) {
	if c.dynamicClient == nil {
		return 0, fmt.Errorf("no dynamic client available")
	}

	gvr := schema.GroupVersionResource{
		Group:    resource.APIGroup,
		Version:  resource.APIVersion,
		Resource: resource.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try to get count with limit=0 (just metadata)
	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
		Limit:          0,
		TimeoutSeconds: &[]int64{3}[0],
	})
	if err != nil {
		return 0, err
	}

	// Get the total count from the list metadata
	if list.GetContinue() != "" {
		// If there's a continue token, we need to count all items
		fullList, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return 0, err
		}
		return len(fullList.Items), nil
	}

	return len(list.Items), nil
}
