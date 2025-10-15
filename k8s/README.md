# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying the k8s-object-explorer application.

## Files

- `deployment.yaml` - Application deployment with 3 replicas, RBAC, and health checks
- `service.yaml` - ClusterIP service to expose the application internally
- `ingress.yaml` - Ingress resource for external access

## Prerequisites

- Kubernetes cluster (1.20+)
- kubectl configured to access your cluster
- Ingress controller installed (e.g., nginx-ingress) for external access

## Quick Deploy

Deploy all resources at once:

```bash
kubectl apply -f k8s/
```

Or deploy individually:

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
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

**Note**: Update the `ClusterRoleBinding` namespace in `deployment.yaml` if using a custom namespace.

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
