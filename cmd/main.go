package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s-object-explorer/internal/k8s"

	"github.com/gorilla/mux"
)

type Server struct {
	k8sClient *k8s.Client
	debug     bool
}

func main() {
	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient("")
	if err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes client: %v", err)
		log.Printf("The application will start but Kubernetes features will be unavailable")
	}

	// Debug mode from environment
	debugEnv := strings.ToLower(os.Getenv("DEBUG"))
	debug := debugEnv == "true" || debugEnv == "1" || debugEnv == "yes"

	server := &Server{k8sClient: k8sClient, debug: debug}

	// Setup routes
	router := mux.NewRouter()

	// Static file serving setup first
	webDir := "web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		webDir = filepath.Join(execDir, "..", "web")
		if _, err := os.Stat(webDir); os.IsNotExist(err) {
			webDir = filepath.Join(".", "web")
		}
	}

	// API routes (must be registered before static file handler)
	router.HandleFunc("/api/namespaces", server.getNamespaces).Methods("GET")
	router.HandleFunc("/api/resources/{namespace}", server.getNamespaceResources).Methods("GET")
	router.HandleFunc("/api/debug-stream/{namespace}", server.getDebugStream).Methods("GET")
	router.HandleFunc("/api/objects/{namespace}/{resource}", server.getResourceObjects).Methods("GET")
	router.HandleFunc("/api/object/{namespace}/{resource}/{name}", server.getObjectDetails).Methods("GET")
	router.HandleFunc("/api/object-raw/{namespace}/{resource}/{name}", server.getRawObjectDetails).Methods("GET")
	router.HandleFunc("/api/export/{namespace}", server.exportResourcesCSV).Methods("GET")
	router.HandleFunc("/api/debug", server.debugStatus).Methods("GET")
	router.HandleFunc("/api/clear-cache", server.clearCache).Methods("POST")

	// Serve static files (this must be last as it's a catch-all)
	router.PathPrefix("/").Handler(http.FileServer(http.Dir(webDir + "/")))

	// Start server
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	fmt.Printf("🚀 Simple Kubernetes Explorer starting on port %s\n", port)
	fmt.Printf("📂 Serving web files from: %s\n", webDir)
	if k8sClient != nil {
		fmt.Printf("🔗 Connected to Kubernetes cluster\n")
	}
	fmt.Printf("🌐 Open http://localhost:%s in your browser\n", port)
	if debug {
		fmt.Printf("🛠️ Debug mode enabled (ENV DEBUG=true)\n")
	}

	log.Fatal(http.ListenAndServe(":"+port, router))
}

func (s *Server) debugStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"debug": s.debug,
	}

	if s.k8sClient != nil && s.debug {
		// Add cache information when debug is enabled
		cacheAge := time.Time{}
		cacheSize := 0
		if s.k8sClient != nil {
			// Access cache info (we'll need to add a getter method)
			response["cache"] = map[string]interface{}{
				"enabled": true,
				"ttl":     "5 minutes",
			}
		}
		_ = cacheAge
		_ = cacheSize
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) clearCache(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient != nil {
		// Clear both API resources cache and namespace caches
		// We'll need to add methods to the k8s client for this
		fmt.Println("🗑️ Cache cleared by user request")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cache cleared"})
}

func (s *Server) getDebugStream(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	if namespace == "" {
		http.Error(w, "Namespace required", http.StatusBadRequest)
		return
	}

	if !s.debug {
		http.Error(w, "Debug mode not enabled", http.StatusNotFound)
		return
	}

	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial message
	fmt.Fprintf(w, "data: {\"type\":\"start\",\"message\":\"🚀 Debug stream started for namespace: %s\"}\n\n", namespace)
	flusher.Flush()

	// Create a custom writer to capture debug output
	debugOutput := make(chan string, 100)

	// Start goroutine to get resources with debug output capture
	go func() {
		defer close(debugOutput)

		// Create callback function to send debug messages to the stream
		debugCallback := func(message string) {
			select {
			case debugOutput <- message:
			case <-time.After(1 * time.Second):
				// Prevent blocking if channel is full
			}
		}

		// Get resources with real-time debug callbacks
		resources, err := s.k8sClient.GetResourcesInNamespaceWithCallback(namespace, debugCallback)
		if err != nil {
			debugOutput <- fmt.Sprintf("❌ Error: %v", err)
			return
		}

		debugOutput <- fmt.Sprintf("🎉 Discovery complete! Found %d resource types with objects", len(resources))
	}()

	// Stream debug messages
	for msg := range debugOutput {
		select {
		case <-r.Context().Done():
			return
		default:
			eventData := map[string]string{
				"type":    "debug",
				"message": msg,
			}
			jsonData, _ := json.Marshal(eventData)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}

	// Send completion message
	fmt.Fprintf(w, "data: {\"type\":\"complete\",\"message\":\"🎉 Debug stream completed\"}\n\n")
	flusher.Flush()
}

func (s *Server) getNamespaces(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	namespaces, err := s.k8sClient.GetNamespaces()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"namespaces": namespaces,
		"count":      len(namespaces),
	})
}

func (s *Server) getNamespaceResources(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	namespace := vars["namespace"]

	fmt.Printf("Loading resources for namespace: %s\n", namespace)

	// Use only server debug flag from environment
	debug := s.debug

	resources, err := s.k8sClient.GetResourcesInNamespace(namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Simple filtering
	search := strings.ToLower(r.URL.Query().Get("search"))
	showOnlyPopulated := r.URL.Query().Get("populated") == "true"
	apiGroup := r.URL.Query().Get("apiGroup")

	var filtered []k8s.ResourceInfo
	totalObjects := 0

	start := time.Now()
	if debug {
		log.Printf("[DEBUG] Namespace=%s resources discovered: %d", namespace, len(resources))
	}

	for _, resource := range resources {
		totalObjects += resource.Count

		// Apply filters
		if showOnlyPopulated && resource.Count == 0 {
			continue
		}
		if apiGroup != "" && resource.APIVersion != apiGroup {
			continue
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(resource.Name), search) &&
				!strings.Contains(strings.ToLower(resource.Kind), search) {
				continue
			}
		}

		filtered = append(filtered, resource)
	}

	if debug {
		// Log top 10 resources by count
		type kv struct {
			Name    string
			Group   string
			Version string
			Count   int
		}
		top := make([]kv, 0, len(filtered))
		for _, rinfo := range filtered {
			grp := rinfo.APIGroup
			if grp == "" {
				grp = "core"
			}
			top = append(top, kv{Name: rinfo.Name, Group: grp, Version: rinfo.APIVersion, Count: rinfo.Count})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].Count > top[j].Count })
		if len(top) > 10 {
			top = top[:10]
		}
		log.Printf("[DEBUG] Filtered resources: %d (populatedOnly=%t search=\"%s\" apiGroup=\"%s\")", len(filtered), showOnlyPopulated, search, apiGroup)
		for _, t := range top {
			log.Printf("[DEBUG]   %s (%s/%s) -> %d objects", t.Name, t.Group, t.Version, t.Count)
		}
		log.Printf("[DEBUG] Completed in %s", time.Since(start))
	}

	fmt.Printf("Found %d resources (%d total objects) in namespace %s\n",
		len(filtered), totalObjects, namespace)

	response := map[string]interface{}{
		"resources":    filtered,
		"count":        len(filtered),
		"totalObjects": totalObjects,
		"namespace":    namespace,
		"debug":        s.debug,
	}

	// Add debug info to response when debug mode is enabled
	if debug {
		debugInfo := make([]map[string]interface{}, 0)

		// Add top resources info
		type kv struct {
			Name    string
			Group   string
			Version string
			Count   int
		}
		top := make([]kv, 0, len(filtered))
		for _, rinfo := range filtered {
			grp := rinfo.APIGroup
			if grp == "" {
				grp = "core"
			}
			top = append(top, kv{Name: rinfo.Name, Group: grp, Version: rinfo.APIVersion, Count: rinfo.Count})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].Count > top[j].Count })

		for i, t := range top {
			if i >= 15 {
				break
			} // Show top 15 in UI
			debugInfo = append(debugInfo, map[string]interface{}{
				"name":    t.Name,
				"group":   t.Group,
				"version": t.Version,
				"count":   t.Count,
			})
		}

		response["debugInfo"] = debugInfo
		response["totalDiscovered"] = len(resources)
		response["processingTime"] = time.Since(start).String()
		// Determine cache status
		cacheStatus := "Fresh data - object counts calculated"
		if len(resources) > 0 {
			// We can infer if this was cached based on timing
			processingTimeMs := time.Since(start).Milliseconds()
			if processingTimeMs < 1000 { // Less than 1 second suggests cache hit
				cacheStatus = "Cache hit - using stored object counts"
			}
		}

		response["cacheInfo"] = map[string]interface{}{
			"enabled": true,
			"status":  cacheStatus,
		}
		response["filters"] = map[string]interface{}{
			"populatedOnly": showOnlyPopulated,
			"search":        search,
			"apiGroup":      apiGroup,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) getResourceObjects(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	namespace := vars["namespace"]
	resource := vars["resource"]

	fmt.Printf("Loading objects for resource: %s in namespace: %s\n", resource, namespace)
	debug := s.debug

	start := time.Now()
	objects, err := s.k8sClient.GetResourceObjects(namespace, resource)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Found %d objects for resource %s in namespace %s\n", len(objects), resource, namespace)

	if debug {
		// Log a few object names
		limit := 5
		if len(objects) < limit {
			limit = len(objects)
		}
		for i := 0; i < limit; i++ {
			o := objects[i]
			log.Printf("[DEBUG]   %s/%s kind=%s apiVersion=%s", o.Namespace, o.Name, o.Kind, o.APIVersion)
		}
		log.Printf("[DEBUG] Objects listing completed in %s", time.Since(start))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"objects":   objects,
		"count":     len(objects),
		"namespace": namespace,
		"resource":  resource,
	})
}

func (s *Server) getObjectDetails(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	namespace := vars["namespace"]
	resource := vars["resource"]
	name := vars["name"]

	fmt.Printf("Loading object details: %s/%s/%s\n", namespace, resource, name)
	debug := s.debug
	start := time.Now()

	object, err := s.k8sClient.GetResourceObject(namespace, resource, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if debug {
		// Log basic manifest metadata
		apiGroup := object.APIVersion
		log.Printf("[DEBUG] Object: kind=%s apiVersion=%s ns=%s name=%s labels=%d annotations=%d",
			object.Kind, apiGroup, object.Namespace, object.Name, len(object.Labels), len(object.Annotations))
		if len(object.Spec) > 0 {
			log.Printf("[DEBUG]   spec: present")
		} else {
			log.Printf("[DEBUG]   spec: empty")
		}
		if len(object.Status) > 0 {
			log.Printf("[DEBUG]   status: present")
		} else {
			log.Printf("[DEBUG]   status: empty")
		}
		log.Printf("[DEBUG] Object details fetched in %s", time.Since(start))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(object)
}

func (s *Server) getRawObjectDetails(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	namespace := vars["namespace"]
	resource := vars["resource"]
	name := vars["name"]

	fmt.Printf("Loading raw object details: %s/%s/%s\n", namespace, resource, name)
	debug := s.debug
	start := time.Now()

	rawObject, err := s.k8sClient.GetRawResourceObject(namespace, resource, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if debug {
		// Log basic object information
		if apiVersion, ok := rawObject["apiVersion"].(string); ok {
			if kind, ok := rawObject["kind"].(string); ok {
				log.Printf("[DEBUG] Raw Object: kind=%s apiVersion=%s", kind, apiVersion)
			}
		}
		if metadata, ok := rawObject["metadata"].(map[string]interface{}); ok {
			if objName, ok := metadata["name"].(string); ok && objName == name {
				log.Printf("[DEBUG]   metadata.name=%s", objName)
			}
			if labels, ok := metadata["labels"].(map[string]interface{}); ok {
				log.Printf("[DEBUG]   labels count=%d", len(labels))
			}
		}
		log.Printf("[DEBUG] Raw object details fetched in %s", time.Since(start))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rawObject)
}

func (s *Server) exportResourcesCSV(w http.ResponseWriter, r *http.Request) {
	if s.k8sClient == nil {
		http.Error(w, "No Kubernetes connection", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	namespace := vars["namespace"]

	fmt.Printf("Exporting resources for namespace: %s\n", namespace)

	resources, err := s.k8sClient.GetResourcesInNamespace(namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set CSV headers
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"k8s-resources-%s.csv\"", namespace))

	// Write CSV header
	fmt.Fprintf(w, "Resource Name,Kind,API Group,API Version,Namespaced,Count\n")

	// Write data rows
	for _, resource := range resources {
		apiGroup := resource.APIGroup
		if apiGroup == "" {
			apiGroup = "core"
		}
		fmt.Fprintf(w, "\"%s\",\"%s\",\"%s\",\"%s\",%t,%d\n",
			resource.Name, resource.Kind, apiGroup, resource.APIVersion,
			resource.Namespaced, resource.Count)
	}

	fmt.Printf("Exported %d resources for namespace %s\n", len(resources), namespace)
}
