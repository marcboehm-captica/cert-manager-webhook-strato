<p align="center">
  <img src="https://raw.githubusercontent.com/cert-manager/cert-manager/d53c0b9270f8cd90d908460d69502694e1838f5f/logo/logo-small.png" height="256" width="256" alt="cert-manager project logo" />
  <img src="https://www.strato.de/_assets/img/png/strato_ag_thumb.png" height="256" alt="strato project logo" />
</p>

# Strato ACME webhook

This inofficial repository contains a cert-manager webhook implementation for the DNS provider STRATO. It enables cert-manager to solve DNS01 ACME challenges using STRATO's DNS services. By integrating with STRATO's API, this webhook allows users to automate the issuance and renewal of TLS certificates for domains managed by STRATO.

The webhook is designed to be deployed as a Kubernetes API service, ensuring secure and restricted access through Kubernetes RBAC. It adheres to cert-manager's webhook interface, making it easy to integrate with existing cert-manager installations.

This implementation includes conformance tests to validate its functionality and ensure compatibility with cert-manager's DNS01 challenge requirements.

## Quickstart Guide

Follow these steps to quickly set up and use the STRATO cert-manager webhook:

### Prerequisites

1. A Kubernetes cluster with cert-manager installed.
2. A Strato account and a domain.
3. It is recommended to keep a backup of your DNS configuration as a precaution.
4. Note: Two-factor authentication is not supported for this integration.
   If this is a feature you would like to see, please open a issue
5. `kubectl` or `helm` installed and configured to access your cluster.

### Installation

#### Using Helm

1. Add the Helm repository:
   ```bash
   helm plugin install https://github.com/aslafy-z/helm-git --version 1.3.0
   helm repo add strato-webhook git+https://github.com/fl0eb/cert-manager-webhook-strato@deploy/strato-webhook
   helm repo update
   ```

3. Create a `values.yaml` file to customize the installation:
   ```yaml
   certManager:
     namespace: <cert-manager namespace>
     serviceAccountName: <cert-manager serviceAccountName>
   ```
   If cert-manager is installed with default values seen below you do not need to provide a values.yaml
   ```yaml
   certManager:
     namespace: cert-manager
     serviceAccountName: cert-manager
   ```

4. Install the webhook using Helm:
   ```bash
   helm install strato-webhook strato-webhook/cert-manager-webhook-strato --namespace cert-manager -f values.yaml
   ```

5. Verify the webhook is running:
   ```bash
   kubectl get pods -n cert-manager
   ```

#### Using kubectl

1. Deploy the webhook to your Kubernetes cluster:
   ```bash
   kubectl apply -f https://github.com/fl0eb/cert-manager-webhook-strato/releases/download/v0.0.1/rendered-manifest.yaml
   ```

2. Verify the webhook is running:
   ```bash
   kubectl get pods -n cert-manager
   ```

### Configuration

1. Create a Kubernetes Secret with your STRATO API credentials:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: strato-dns-credentials
     namespace: cert-manager
   type: Opaque
   data:
     identity: <base64-encoded-identity>
     password: <base64-encoded-password>
   ```
   Apply the secret:
   ```bash
   kubectl apply -f strato-dns-credentials.yaml
   ```
   Alternatively, you can create the secret directly from literals using `kubectl`:

   ```bash
   kubectl create secret generic strato-dns-credentials \
     --namespace cert-manager \
     --from-literal=identity=<your-identity> \
     --from-literal=password=<your-password>
   ```

2. Configure an Issuer or ClusterIssuer to use the webhook:
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: strato-issuer
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: your-email@example.com
       privateKeySecretRef:
         name: strato-issuer-account-key
       solvers:
       - dns01:
           webhook:
             groupName: fl0eb.github.com
             solverName: strato
             config:
               secretName: strato-dns-credentials
               # You should be able to change this to other regions if required (.nl .fr .es ...)
               api: "https://www.strato.de/apps/CustomerService" 
               # For the following values you can check:
               # Strato Service Portal > Ihre Pakete > Paketübersicht
               domain: "example.com"   # The root domain (Kennung) this issuer will modify
               order: "1234567"      # The package order id (Auftragsnummer) the domain belongs to
   ```
   Apply the issuer:
   ```bash
   kubectl apply -f strato-issuer.yaml
   ```

### Requesting a Certificate

1. Create a Certificate resource:
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: example-com-tls
     namespace: default
   spec:
     secretName: example-com-tls
     issuerRef:
       name: strato-issuer
       kind: ClusterIssuer
     commonName: example.com
     dnsNames:
     - "*.example.com"
   ```
   Apply the certificate:
   ```bash
   kubectl apply -f certificate.yaml
   ```

2. Verify the certificate is issued:
   ```bash
   kubectl describe certificate example-com-tls
   ```

## Special Thanks

This project was inspired by the following repositories:

- [strato-certbot](https://github.com/Buxdehuda/strato-certbot)
- [cert-manager-webhook-infomaniak](https://github.com/Infomaniak/cert-manager-webhook-infomaniak)

