# WORK IN PROGRESS

## description

cert-manager webhook is a plugin for bluecat to create letsencrypt certificates.

## usage

* deploy cert-manager, see [here](https://github.com/jetstack/cert-manager)
* create ClusterIssuer & Certificate, see examples
* deploy plugin with helm, see [here](https://github.com/containeroo/helm-charts/tree/master/charts/cert-manager-webhook-bluecat)

## Examples

### ClusterIssuer

```bash
cat <<EOF | kubectl apply -f -
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    email: acme@example.com
    privateKeySecretRef:
      name: le-staging-account-key
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    solvers:
    - selector: {}
      dns01:
        webhook:
          groupName: acme.example.com
          solverName: infomaniak
          config:
            apiTokenSecretRef:
              name: example-api-credentials
              key: api-token
EOF
```

### Certificate

Create a Certificate, the issued cert will be stored in the specified Secret (keys tls.crt & tls.key):

```bash
cat <<EOF | kubectl apply -f -
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-example-com
spec:
  secretName: test-example-com-tls
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
  dnsNames:
  - test.example.com
EOF
```

```bash
kubectl get secret test-example-com-tls -o json | jq -r '.data."tls.crt"' | base64 -d | openssl x509 -text -noout | grep Subject: Subject: CN = test.example.com
```

Thanks to:

* https://github.com/Infomaniak/cert-manager-webhook-infomaniak
* https://github.com/go-acme/lego
