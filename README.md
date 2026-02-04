# devops-projects

# ArgoCD behind NGINX Gateway Fabric (MetalLB + cert-manager) – Local Cluster Guide

This document captures the **final working setup**, the **why behind each component**, and the **key lessons learned** while exposing ArgoCD through **NGINX Gateway Fabric** on a **local Kubernetes cluster using MetalLB**.

---

## 1. Goal

Expose **ArgoCD UI** securely at:

```
https://argo.654537853.xyz
```

Using:
- Gateway API (NGINX Gateway Fabric)
- MetalLB for LoadBalancer IPs
- cert-manager with Let's Encrypt (staging)
- Cloudflare DNS
- Local (non-cloud) Kubernetes cluster

---

## 2. High-Level Architecture

```
Browser
  |
  |  argo.654537853.xyz
  v
MetalLB External IP (Gateway Service)
  |
NGINX Gateway Fabric (Gateway)
  |
HTTPRoute
  |
ArgoCD Server Service
```

---

## 3. Why Two Services Exist (Important!)

```
ngf-nginx-gateway-fabric   → controller / management service
homecluster-gateway-nginx → data-plane service (THIS ONE MATTERS)
```

### ✅ Traffic Flow Uses:
```
homecluster-gateway-nginx (LoadBalancer)
```

### ❌ The following is NOT used for app traffic:
```
ngf-nginx-gateway-fabric
```

This confusion caused most of the early troubleshooting.

---

## 4. GatewayClass (Created by Helm)

You never manually created this:

```bash
kubectl get gatewayclass nginx
```

It is installed by the `nginx-gateway-fabric` Helm chart and shared across all Gateways.

---

## 5. Gateway Definition (Working)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: homecluster-gateway
  namespace: nginx-gateway
spec:
  gatewayClassName: nginx
  listeners:
  - name: argo-http
    port: 80
    protocol: HTTP
    hostname: "argo.654537853.xyz"

  - name: argo-https
    port: 443
    protocol: HTTPS
    hostname: "argo.654537853.xyz"
    tls:
      mode: Terminate
      certificateRefs:
      - name: argo-654537853-xyz-tls
```

---

## 6. Why a Certificate Resource Was Needed (Tutorial Didn’t Show This)

The tutorial relied on this annotation:

```yaml
cert-manager.io/cluster-issuer: letsencrypt-prod
```

In **your environment**, cert-manager did **not** automatically create the Certificate, so you explicitly defined it:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: argo-654537853-xyz
  namespace: nginx-gateway
spec:
  secretName: argo-654537853-xyz-tls
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
  dnsNames:
  - argo.654537853.xyz
```

➡️ This secret **was correctly created** and later confirmed via `curl`.

---

## 7. cert-manager + Cloudflare Notes

- The error about `cloudflare-api-token` was due to namespace mismatch
- cert-manager eventually issued a valid **Let’s Encrypt STAGING** certificate
- STAGING certs are:
  - Cryptographically valid
  - Browser-untrusted (expected)
  - Perfect for local testing

---

## 8. MetalLB IP Confusion (Critical Clarity)

MetalLB dynamically assigns IPs from its pool.

That’s why you saw IPs change:

```
192.168.0.240 → 192.168.0.241
```

### Correct IP to use for DNS:
```
EXTERNAL-IP of homecluster-gateway-nginx
```

Verify with:
```bash
kubectl get svc -n nginx-gateway
```

---

## 9. Why Ping Didn’t Work (But curl Did)

MetalLB does **not guarantee ICMP (ping)**.

This is expected:
```bash
ping <MetalLB-IP>  ❌
curl http(s)://<MetalLB-IP> ✅
```

---

## 10. Redirect Loop Root Cause (Key Lesson)

### Symptom
```
ERR_TOO_MANY_REDIRECTS
```

### Cause
- ArgoCD server runs HTTPS by default
- Gateway was terminating TLS
- Result: HTTPS → HTTPS → HTTPS loop

---

## 11. Final Fix (What Actually Worked)

### Option Used (Matches Tutorial)

```bash
kubectl patch configmap argocd-cmd-params-cm   -n argocd   --patch '{"data":{"server.insecure":"true"}}'
```

This is **equivalent** to adding:

```
--insecure
```

To the argocd-server container.

### Why This Works
- Gateway handles TLS
- ArgoCD serves plain HTTP internally
- No redirect loop

---

## 12. Final Verification

### Curl Test (Authoritative)

```bash
curl -vk https://argo.654537853.xyz   --resolve argo.654537853.xyz:443:<MetalLB-IP>
```

✅ Result:
- Correct hostname
- Correct certificate
- HTTP 307 redirect handled
- No TLS errors at Gateway

---

## 13. Browser Result

- Firefox / Chrome load ArgoCD UI
- Certificate warning (expected – STAGING)
- Functional UI

---

## 14. Key Takeaways

- Gateway creates the Service (not YAML)
- Only **one** LoadBalancer IP matters
- cert-manager behavior differs locally
- Redirect loops = TLS termination mismatch
- STAGING certs are correct for testing
- MetalLB IPs can and will change

---

## 15. Next Improvements (Optional)

- Switch to Let’s Encrypt PROD
- Add HTTP → HTTPS redirect at Gateway
- Automate DNS updates
- Try TLS Passthrough (if supported)

---

✅ **This setup is correct, functional, and production-shaped for a local cluster.**
