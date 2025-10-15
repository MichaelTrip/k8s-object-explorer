# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying the k8s-object-explorer application.

## Files

- `rbac.yaml` - ServiceAccount, ClusterRole, and ClusterRoleBinding (read-only access)
- `deployment.yaml` - Application deployment with 3 replicas, RBAC, and health checks (includes RBAC inline)
- `service.yaml` - ClusterIP service to expose the application internally
- `ingress.yaml` - Ingress resource for external access

## Prerequisites

- Kubernetes cluster (1.20+)
- kubectl configured to access your cluster
- Ingress controller installed (e.g., nginx-ingress) for external access

## Quick Deploy

### Option 1: All Resources (Recommended)
Deploy all resources including RBAC at once:

```bash
kubectl apply -f k8s/
```

### Option 2: Selective Deployment
Deploy individually for more control:

```bash
# Option A: Use standalone RBAC file
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml  # Optional

# Option B: deployment.yaml includes RBAC inline
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml  # Optional
```

## Configuration

### Ingress

Before deploying the ingress, update the host in `ingress.yaml`:

```yaml
rules:
- host: k8s-explorer.example.com  # Change this to your actual domain
```

### TLS (Optional)

To enable HTTPS, uncomment the TLS section in `ingress.yaml` and create a TLS secret:

```bash
kubectl create secret tls k8s-explorer-tls --cert=path/to/tls.crt --key=path/to/tls.key
```

Or use cert-manager:

```bash
# Uncomment the cert-manager annotation in ingress.yaml
# cert-manager.io/cluster-issuer: "letsencrypt-prod"
```

### Custom Namespace

Deploy to a custom namespace:

```bash
kubectl create namespace k8s-explorer
kubectl apply -f k8s/ -n k8s-explorer
```

**Note**: Update the `ClusterRoleBinding` namespace in `deployment.yaml` or `rbac.yaml` if using a custom namespace.

## RBAC Permissions

The application requires **read-only** access to discover and count resources across the cluster:

### ServiceAccount
- Name: `k8s-object-explorer`
- Namespace: `default` (or custom namespace)

### ClusterRole Permissions
```yaml
rules:
  # Read access to all API resources for discovery
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list"]
  
  # Specific access to namespaces
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list"]
  
  # Access to API discovery endpoints
  - nonResourceURLs: ["/api", "/api/*", "/apis", "/apis/*"]
    verbs: ["get"]
```

### Why These Permissions?
- **`get` and `list` on all resources**: Required to discover available API resources and count objects
- **No write access**: Application is read-only and cannot modify any cluster resources
- **Cluster-wide access**: Needed to discover resources across all namespaces
- **API discovery**: Required to enumerate available resource types dynamically

### Security Note
While the permissions are cluster-wide, they are **read-only**. The application:
- ✅ Cannot create, update, or delete any resources
- ✅ Cannot access secrets' values (only lists that they exist)
- ✅ Runs as non-root user (UID 1000)
- ✅ Has dropped all Linux capabilities

## Accessing the Application

1. **Port Forward (for testing)**:
   ```bash
   kubectl port-forward service/k8s-object-explorer 8080:80
   ```
   Then visit `http://localhost:8080`

2. **Via Ingress** (production):
   - Ensure your ingress controller is installed
   - Update your DNS to point to the ingress controller's external IP
   - Visit `http://your-domain.com`

## Monitoring

Check the deployment status:

```bash
kubectl get deployments
kubectl get pods
kubectl get services
kubectl get ingress
```

View logs:

```bash
kubectl logs -l app=k8s-object-explorer
kubectl logs -l app=k8s-object-explorer --tail=100 -f
```

## Scaling

Scale the deployment:

```bash
kubectl scale deployment k8s-object-explorer --replicas=5
```

## RBAC Permissions

The application requires read-only access to all Kubernetes resources. The included `ClusterRole` provides:

- `get` and `list` permissions for all API resources
- Access to all namespaces

This is necessary for the application to discover and display resources across the cluster.

## Troubleshooting

### Pods not starting
```bash
kubectl describe pod -l app=k8s-object-explorer
kubectl logs -l app=k8s-object-explorer
```

### Permission errors
Ensure the `ServiceAccount`, `ClusterRole`, and `ClusterRoleBinding` are properly created:
```bash
kubectl get serviceaccount k8s-object-explorer
kubectl get clusterrole k8s-object-explorer
kubectl get clusterrolebinding k8s-object-explorer
```

### Ingress not working
Check ingress controller logs and ensure your DNS is properly configured:
```bash
kubectl get ingress k8s-object-explorer
kubectl describe ingress k8s-object-explorer
```

## Cleanup

Remove all resources:

```bash
kubectl delete -f k8s/
```

Or individually:

```bash
kubectl delete deployment k8s-object-explorer
kubectl delete service k8s-object-explorer
kubectl delete ingress k8s-object-explorer
kubectl delete serviceaccount k8s-object-explorer
kubectl delete clusterrole k8s-object-explorer
kubectl delete clusterrolebinding k8s-object-explorer
```
