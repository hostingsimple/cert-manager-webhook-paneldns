# cert-manager DNS-01 webhook for PanelDNS

[![Latest Release](https://img.shields.io/github/v/release/hostingsimple/cert-manager-webhook-paneldns)](https://github.com/hostingsimple/cert-manager-webhook-paneldns/releases/latest) [![Release](https://github.com/hostingsimple/cert-manager-webhook-paneldns/actions/workflows/release.yml/badge.svg)](https://github.com/hostingsimple/cert-manager-webhook-paneldns/actions/workflows/release.yml) [![License](https://img.shields.io/github/license/hostingsimple/cert-manager-webhook-paneldns)](LICENSE) [![CI](https://github.com/hostingsimple/cert-manager-webhook-paneldns/actions/workflows/ci.yml/badge.svg)](https://github.com/hostingsimple/cert-manager-webhook-paneldns/actions/workflows/ci.yml) ![Kubernetes](https://img.shields.io/badge/Kubernetes-DNS--01-326CE5?logo=kubernetes&logoColor=white)

A [cert-manager](https://cert-manager.io) DNS-01 webhook solver that uses [PanelDNS](https://paneldns.com) to complete ACME challenges. Works with any Kubernetes ingress controller (nginx-ingress, Traefik, Istio, Gateway API) since cert-manager sits below all of them.

## Prerequisites

- Kubernetes 1.25+
- cert-manager v1.12+
- Helm 3
- A PanelDNS API token with `zones:read records:read records:write` scopes

## Installation

```sh
# 1. Create the API token secret
kubectl create secret generic paneldns-credentials \
  --from-literal=token=dnsm_xxxx \
  -n cert-manager

# 2. Install the webhook
helm install cert-manager-webhook-paneldns \
  oci://ghcr.io/veeau/charts/cert-manager-webhook-paneldns \
  --namespace cert-manager

# 3. Create a ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod-key
    solvers:
    - dns01:
        webhook:
          groupName: acme.paneldns.com
          solverName: paneldns
          config:
            apiUrl: https://app.paneldns.com
            apiTokenSecretRef:
              name: paneldns-credentials
              key: token
EOF
```

## Requesting a certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-example-com
  namespace: default
spec:
  secretName: wildcard-example-com-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - "*.example.com"
    - "example.com"
```

## Self-hosted PanelDNS

```yaml
# In ClusterIssuer spec.acme.solvers[0].dns01.webhook.config:
config:
  apiUrl: https://dns.yourdomain.com
  apiTokenSecretRef:
    name: paneldns-credentials
    key: token
```

## Configuration reference

| Field | Default | Description |
|---|---|---|
| `apiUrl` | `https://app.paneldns.com` | PanelDNS instance URL |
| `apiTokenSecretRef.name` | — | Kubernetes Secret name containing the token |
| `apiTokenSecretRef.namespace` | issuer namespace | Secret namespace (defaults to issuer's namespace) |
| `apiTokenSecretRef.key` | `token` | Key within the Secret |

## License

MIT
