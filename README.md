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

## Build

```bash
docker build \
--tag=ghcr.io/dazwilkin/akri-webhook:$(git rev-parse HEAD) \
--file=./Dockerfile \
.
```

## Run (locally)

```bash
docker run \
--rm --interactive --tty \
--volume=${PWD}/secrets:/secrets \
ghcr.io/dazwilkin/akri-webhook:8212d5d516920a8678f395358ec1a4852653c55e \
  --tls-crt-file=/secrets/localhost.crt \
  --tls-key-file=/secrets/localhost.key \
  --port=8443
```

## Certificates

```bash
DIR="${PWD}/secrets"
NAME="webhook"
NAMESPACE="default"

openssl req -nodes -new -x509 -keyout ${DIR}/ca.key -out ${DIR}/ca.crt -subj "/CN=Akri Webhook"
openssl genrsa -out ${DIR}/${NAME}.key 2048


openssl req -new -key ${DIR}/${NAME}.key -subj "/CN=${NAME}.${NAMESPACE}.svc" \
| openssl x509 -req -CA ${DIR}/ca.crt -CAkey ${DIR}/ca.key -CAcreateserial -out ${DIR}/${NAME}.crt
```

Then:

```bash
CA_BUNDLE=$(cat ./secrets/ca.crt | openssl base64 -A)
```

Then:

```bash
kubectl create secret tls ${NAME} \
--cert=${DIR}/${NAME}.crt \
--key=${DIR}/${NAME}.key
```


## Deploy

But:

```bash
sed "s|CABUNDLE|${CA_BUNDLE}|g" ./validatingwebhook.yaml \
| kubectl apply --filename=-
```

Generating errors:

```console
error: error validating "STDIN": error validating data: [ValidationError(ValidatingWebhookConfiguration.webhooks[0]): missing required field "sideEffects" in io.k8s.api.admissionregistration.v1.ValidatingWebhook, ValidationError(ValidatingWebhookConfiguration.webhooks[0]): missing required field "admissionReviewVersions" in io.k8s.api.admissionregistration.v1.ValidatingWebhook]; if you choose to ignore these errors, turn validation off with --validate=false
```

See: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#validatingwebhook-v1-admissionregistration-k8s-io
