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

## Certificates #1 (Monday)

```bash
DIR="${PWD}/secrets"
SERVICE="webhook"
NAMESPACE="default"

openssl req -nodes -new -x509 -keyout ${DIR}/ca.key -out ${DIR}/ca.crt -subj "/CN=Akri Webhook"
openssl genrsa -out ${DIR}/${SERVICE}.key 2048


openssl req -new -key ${DIR}/${SERVICE}.key -subj "/CN=${SERVICE}.${NAMESPACE}.svc" \
| openssl x509 -req -CA ${DIR}/ca.crt -CAkey ${DIR}/ca.key -CAcreateserial -out ${DIR}/${SERVICE}.crt
```

Then:

```bash
CA_BUNDLE=$(cat ./secrets/ca.crt | openssl base64 -A)
```

Then:

```bash
kubectl create secret tls ${SERVICE} \
--cert=${DIR}/${SERVICE}.crt \
--key=${DIR}/${SERVICE}.key
```

## Certificates #2 (Tuesday)

```bash
DIR=${PWD}/secrets
SERVICE="tuesday"
NAMESPACE="default"

FILENAME="${DIR}/${SERVICE}.${NAMESPACE}"

openssl req -new -sha256 -newkey rsa:2048 -keyout ${FILENAME}.key -out ${FILENAME}.csr -nodes -subj "/CN=${SERVICE}.${NAMESPACE}"

cat <<EOF | kubectl apply --filename -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${SERVICE}.${NAMESPACE}
spec:
  request: $(cat ${FILENAME}.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

kubectl certificate approve ${SERVICE}.${NAMESPACE}

kubectl get csr ${SERVICE}.${NAMESPACE} -o jsonpath='{.status.certificate}' \
| base64 --decode > ${FILENAME}.crt

kubectl create secret tls ${SERVICE} \
--namespace=${NAMESPACE} \
--cert=${FILENAME}.crt \
--key=${FILENAME}.key

# kubectl create secret generic ${SERVICE} \
# --namespace=${NAMESPACE} \
# --from-file=key.pem=${FILENAME}.key \
# --from-file=crt.pem=${FILENAME}.crt

cat ./webhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|$(cat ${FILENAME}.crt | base64 --wrap=0)|g" \
| kubectl apply --filename=-
```

Yields:

```bash
ls -la secrets

tuesday.default.crt
tuesday.default.csr
tuesday.default.key
```


## Deploy

But:

```bash
cat ./webhook.deployment.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=-

cat ./webhook.service.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=-

cat ./validatingwebhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|${CA_BUNDLE}|g" \
| kubectl apply --filename=-
```
