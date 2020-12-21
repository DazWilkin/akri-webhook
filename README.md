# Akri: ValidatingAdmissionWebhook for Configurations (CRD)

See: https://github.com/deislabs/akri/issues/180 Specifically: https://github.com/deislabs/akri/issues/180#issuecomment-748540637

References:

+ Kubenretes [A Guide to Kubernetes Admission Controllers](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
+ Kubernetes [Admission Controllers: ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook)
+ Kubernetes' E2E tests [webhook](https://github.com/kubernetes/kubernetes/blob/v1.13.0/test/images/webhook/main.go)
+ Kubernetes API Reference [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#validatingwebhookconfiguration-v1-admissionregistration-k8s-io)

```bash
openssl req -x509 -nodes -newkey rsa:4096 -keyout localhost.key -out localhost.crt -days 365 -subj "/CN=localhost"
```


Spec

```YAML
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: akri-configuration-webhook
  labels:
    project: akri
    component: validating-webhook
    language: golang
webhooks:
- name: akri-configuration-webhook
  clientConfig:
    service:
      name: akri-configuration-webhook
      namespace: default
      path: "/mutate"
    caBundle: ${CA_BUNDLE}
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
  namespaceSelector:
    matchLabels:
      project: akri
```
