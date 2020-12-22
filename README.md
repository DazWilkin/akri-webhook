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

# echo "
# [req]
# req_extensions = v3_req
# distinguished_name = req_distinguished_name
# [req_distinguished_name]
# [v3_req]
# basicConstraints = CA:FALSE
# keyUsage = nonRepudiation, digitalSignature, keyEncipherment
# extendedKeyUsage = serverAuth
# subjectAltName = @alt_names
# [alt_names]
# DNS.1 = ${SERVICE}
# DNS.2 = ${SERVICE}.${NAMESPACE}
# DNS.3 = ${SERVICE}.${NAMESPACE}.svc
# " > ${FILENAME}.conf

# openssl genrsa \
# -out ${FILENAME}.key 2048

# echo "
# [req]
# default_bits = 2048
# prompt = no
# default_md = sha256
# req_extensions = req_ext
# distinguished_name = dn
# [dn]
# O=.
# CN=${SERVICE}.${NAMESPACE}.svc
# [req_ext]
# subjectAltName = @alt_names
# [alt_names]
# DNS.1 = ${SERVICE}
# DNS.2 = ${SERVICE}.${NAMESPACE}
# DNS.3 = ${SERVICE}.${NAMESPACE}.svc
# " > ${FILENAME}.conf

openssl req \
-new \
-sha256 \
-newkey rsa:2048 \
-keyout ${FILENAME}.key \
-out ${FILENAME}.csr \
-nodes \
-subj "/CN=${SERVICE}.${NAMESPACE}.svc" 
# \
# -config ${FILENAME}.conf

echo "
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${SERVICE}.${NAMESPACE}
spec:
  groups:
  - system:authenticated
  request: $(cat ${FILENAME}.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
" | kubectl apply --filename=-

kubectl certificate approve ${SERVICE}.${NAMESPACE}

kubectl get csr ${SERVICE}.${NAMESPACE} \
--output=jsonpath='{.status.certificate}' \
| base64 --decode > ${FILENAME}.crt

kubectl create secret tls ${SERVICE} \
--namespace=${NAMESPACE} \
--cert=${FILENAME}.crt \
--key=${FILENAME}.key

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

## Deleting

```bash
cat ./webhook.deployment.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl delete --filename=-

cat ./webhook.service.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl delete --filename=-

cat ./validatingwebhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|${CA_BUNDLE}|g" \
| kubectl delete --filename=-
```

Or, more succinctly:

```bash
kubectl delete deployment/${SERVICE}
kubectl delete service/${SERVICE}
kubectl delete validatingwebhookconfiguration/${SERVICE}
```

## Debugging

```bash
REPO="ghcr.io/dazwilkin"
VERS="v0.0.44-amd64"

sudo microk8s.helm3 install akri ./akri/deployment/helm --set imagePullSecrets[0].name=ghcr --set agent.image.repository=${REPO}/agent --set agent.image.tag=${VERS} --set controller.image.repository=${REPO}/controller --set controller.image.tag=${VERS}

kubectl apply --filename=./zeroconf.yaml

kubectl run curl -it --rm --image=curlimages/curl -- sh
curl \
--insecure \
--request POST \
--header "Content-Type: application/json:" \
https://tuesday.default.svc/validate
```


And:

```console
{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1beta1","request":{"uid":"18956111-376b-4bce-8ffd-0819739d0383","kind":{"group":"coordination.k8s.io","version":"v1","kind":"Lease"},"resource":{"group":"coordination.k8s.io","version":"v1","resource":"leases"},"requestKind":{"group":"coordination.k8s.io","version":"v1","kind":"Lease"},"requestResource":{"group":"coordination.k8s.io","version":"v1","resource":"leases"},"name":"kube-controller-manager","namespace":"kube-system","operation":"UPDATE","userInfo":{"username":"system:kube-controller-manager","uid":"controller","groups":["system:authenticated"]},"object":{"kind":"Lease","apiVersion":"coordination.k8s.io/v1","metadata":{"name":"kube-controller-manager","namespace":"kube-system","selfLink":"/apis/coordination.k8s.io/v1/namespaces/kube-system/leases/kube-controller-manager","uid":"96ba640f-98a8-4d8b-b41a-b8f04bd61704","resourceVersion":"583224","creationTimestamp":"2020-10-26T17:03:33Z","managedFields":[{"manager":"kube-controller-manager","operation":"Update","apiVersion":"coordination.k8s.io/v1","time":"2020-12-11T22:07:59Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{"f:acquireTime":{},"f:holderIdentity":{},"f:leaseDurationSeconds":{},"f:leaseTransitions":{},"f:renewTime":{}}}}]},"spec":{"holderIdentity":"akri_2aebdcf5-bf34-444f-b3dd-117873b0cdef","leaseDurationSeconds":15,"acquireTime":"2020-12-22T21:55:56.000000Z","renewTime":"2020-12-22T22:01:51.937572Z","leaseTransitions":54}},"oldObject":{"kind":"Lease","apiVersion":"coordination.k8s.io/v1","metadata":{"name":"kube-controller-manager","namespace":"kube-system","uid":"96ba640f-98a8-4d8b-b41a-b8f04bd61704","resourceVersion":"583224","creationTimestamp":"2020-10-26T17:03:33Z"},"spec":{"holderIdentity":"akri_2aebdcf5-bf34-444f-b3dd-117873b0cdef","leaseDurationSeconds":15,"acquireTime":"2020-12-22T21:55:56.000000Z","renewTime":"2020-12-22T22:01:49.892509Z","leaseTransitions":54}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}}
I1222 22:01:52.043073       1 main.go:91] [serve] Request:
{TypeMeta:{Kind:AdmissionReview APIVersion:admission.k8s.io/v1beta1} Request:&AdmissionRequest{UID:18956111-376b-4bce-8ffd-0819739d0383,Kind:coordination.k8s.io/v1, Kind=Lease,Resource:{coordination.k8s.io v1 leases},SubResource:,Name:kube-controller-manager,Namespace:kube-system,Operation:UPDATE,UserInfo:{system:kube-controller-manager controller [system:authenticated] map[]},Object:{[123 34 107 105 110 100 34 58 34 76 101 97 115 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 109 101 116 97 100 97 116 97 34 58 123 34 110 97 109 101 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 110 97 109 101 115 112 97 99 101 34 58 34 107 117 98 101 45 115 121 115 116 101 109 34 44 34 115 101 108 102 76 105 110 107 34 58 34 47 97 112 105 115 47 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 47 110 97 109 101 115 112 97 99 101 115 47 107 117 98 101 45 115 121 115 116 101 109 47 108 101 97 115 101 115 47 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 117 105 100 34 58 34 57 54 98 97 54 52 48 102 45 57 56 97 56 45 52 100 56 98 45 98 52 49 97 45 98 56 102 48 52 98 100 54 49 55 48 52 34 44 34 114 101 115 111 117 114 99 101 86 101 114 115 105 111 110 34 58 34 53 56 51 50 50 52 34 44 34 99 114 101 97 116 105 111 110 84 105 109 101 115 116 97 109 112 34 58 34 50 48 50 48 45 49 48 45 50 54 84 49 55 58 48 51 58 51 51 90 34 44 34 109 97 110 97 103 101 100 70 105 101 108 100 115 34 58 91 123 34 109 97 110 97 103 101 114 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 111 112 101 114 97 116 105 111 110 34 58 34 85 112 100 97 116 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 116 105 109 101 34 58 34 50 48 50 48 45 49 50 45 49 49 84 50 50 58 48 55 58 53 57 90 34 44 34 102 105 101 108 100 115 84 121 112 101 34 58 34 70 105 101 108 100 115 86 49 34 44 34 102 105 101 108 100 115 86 49 34 58 123 34 102 58 115 112 101 99 34 58 123 34 102 58 97 99 113 117 105 114 101 84 105 109 101 34 58 123 125 44 34 102 58 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 123 125 44 34 102 58 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 123 125 44 34 102 58 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 123 125 44 34 102 58 114 101 110 101 119 84 105 109 101 34 58 123 125 125 125 125 93 125 44 34 115 112 101 99 34 58 123 34 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 34 97 107 114 105 95 50 97 101 98 100 99 102 53 45 98 102 51 52 45 52 52 52 102 45 98 51 100 100 45 49 49 55 56 55 51 98 48 99 100 101 102 34 44 34 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 49 53 44 34 97 99 113 117 105 114 101 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 49 58 53 53 58 53 54 46 48 48 48 48 48 48 90 34 44 34 114 101 110 101 119 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 50 58 48 49 58 53 49 46 57 51 55 53 55 50 90 34 44 34 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 53 52 125 125] <nil>},OldObject:{[123 34 107 105 110 100 34 58 34 76 101 97 115 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 109 101 116 97 100 97 116 97 34 58 123 34 110 97 109 101 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 110 97 109 101 115 112 97 99 101 34 58 34 107 117 98 101 45 115 121 115 116 101 109 34 44 34 117 105 100 34 58 34 57 54 98 97 54 52 48 102 45 57 56 97 56 45 52 100 56 98 45 98 52 49 97 45 98 56 102 48 52 98 100 54 49 55 48 52 34 44 34 114 101 115 111 117 114 99 101 86 101 114 115 105 111 110 34 58 34 53 56 51 50 50 52 34 44 34 99 114 101 97 116 105 111 110 84 105 109 101 115 116 97 109 112 34 58 34 50 48 50 48 45 49 48 45 50 54 84 49 55 58 48 51 58 51 51 90 34 125 44 34 115 112 101 99 34 58 123 34 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 34 97 107 114 105 95 50 97 101 98 100 99 102 53 45 98 102 51 52 45 52 52 52 102 45 98 51 100 100 45 49 49 55 56 55 51 98 48 99 100 101 102 34 44 34 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 49 53 44 34 97 99 113 117 105 114 101 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 49 58 53 53 58 53 54 46 48 48 48 48 48 48 90 34 44 34 114 101 110 101 119 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 50 58 48 49 58 52 57 46 56 57 50 53 48 57 90 34 44 34 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 53 52 125 125] <nil>},DryRun:*false,Options:{[123 34 107 105 110 100 34 58 34 85 112 100 97 116 101 79 112 116 105 111 110 115 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 109 101 116 97 46 107 56 115 46 105 111 47 118 49 34 125] <nil>},RequestKind:coordination.k8s.io/v1, Kind=Lease,RequestResource:coordination.k8s.io/v1, Resource=leases,RequestSubResource:,} Response:nil}
I1222 22:01:52.046508       1 main.go:92] [serve] Runtime Object:
&AdmissionReview{Request:&AdmissionRequest{UID:18956111-376b-4bce-8ffd-0819739d0383,Kind:coordination.k8s.io/v1, Kind=Lease,Resource:{coordination.k8s.io v1 leases},SubResource:,Name:kube-controller-manager,Namespace:kube-system,Operation:UPDATE,UserInfo:{system:kube-controller-manager controller [system:authenticated] map[]},Object:{[123 34 107 105 110 100 34 58 34 76 101 97 115 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 109 101 116 97 100 97 116 97 34 58 123 34 110 97 109 101 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 110 97 109 101 115 112 97 99 101 34 58 34 107 117 98 101 45 115 121 115 116 101 109 34 44 34 115 101 108 102 76 105 110 107 34 58 34 47 97 112 105 115 47 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 47 110 97 109 101 115 112 97 99 101 115 47 107 117 98 101 45 115 121 115 116 101 109 47 108 101 97 115 101 115 47 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 117 105 100 34 58 34 57 54 98 97 54 52 48 102 45 57 56 97 56 45 52 100 56 98 45 98 52 49 97 45 98 56 102 48 52 98 100 54 49 55 48 52 34 44 34 114 101 115 111 117 114 99 101 86 101 114 115 105 111 110 34 58 34 53 56 51 50 50 52 34 44 34 99 114 101 97 116 105 111 110 84 105 109 101 115 116 97 109 112 34 58 34 50 48 50 48 45 49 48 45 50 54 84 49 55 58 48 51 58 51 51 90 34 44 34 109 97 110 97 103 101 100 70 105 101 108 100 115 34 58 91 123 34 109 97 110 97 103 101 114 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 111 112 101 114 97 116 105 111 110 34 58 34 85 112 100 97 116 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 116 105 109 101 34 58 34 50 48 50 48 45 49 50 45 49 49 84 50 50 58 48 55 58 53 57 90 34 44 34 102 105 101 108 100 115 84 121 112 101 34 58 34 70 105 101 108 100 115 86 49 34 44 34 102 105 101 108 100 115 86 49 34 58 123 34 102 58 115 112 101 99 34 58 123 34 102 58 97 99 113 117 105 114 101 84 105 109 101 34 58 123 125 44 34 102 58 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 123 125 44 34 102 58 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 123 125 44 34 102 58 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 123 125 44 34 102 58 114 101 110 101 119 84 105 109 101 34 58 123 125 125 125 125 93 125 44 34 115 112 101 99 34 58 123 34 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 34 97 107 114 105 95 50 97 101 98 100 99 102 53 45 98 102 51 52 45 52 52 52 102 45 98 51 100 100 45 49 49 55 56 55 51 98 48 99 100 101 102 34 44 34 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 49 53 44 34 97 99 113 117 105 114 101 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 49 58 53 53 58 53 54 46 48 48 48 48 48 48 90 34 44 34 114 101 110 101 119 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 50 58 48 49 58 53 49 46 57 51 55 53 55 50 90 34 44 34 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 53 52 125 125] <nil>},OldObject:{[123 34 107 105 110 100 34 58 34 76 101 97 115 101 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 99 111 111 114 100 105 110 97 116 105 111 110 46 107 56 115 46 105 111 47 118 49 34 44 34 109 101 116 97 100 97 116 97 34 58 123 34 110 97 109 101 34 58 34 107 117 98 101 45 99 111 110 116 114 111 108 108 101 114 45 109 97 110 97 103 101 114 34 44 34 110 97 109 101 115 112 97 99 101 34 58 34 107 117 98 101 45 115 121 115 116 101 109 34 44 34 117 105 100 34 58 34 57 54 98 97 54 52 48 102 45 57 56 97 56 45 52 100 56 98 45 98 52 49 97 45 98 56 102 48 52 98 100 54 49 55 48 52 34 44 34 114 101 115 111 117 114 99 101 86 101 114 115 105 111 110 34 58 34 53 56 51 50 50 52 34 44 34 99 114 101 97 116 105 111 110 84 105 109 101 115 116 97 109 112 34 58 34 50 48 50 48 45 49 48 45 50 54 84 49 55 58 48 51 58 51 51 90 34 125 44 34 115 112 101 99 34 58 123 34 104 111 108 100 101 114 73 100 101 110 116 105 116 121 34 58 34 97 107 114 105 95 50 97 101 98 100 99 102 53 45 98 102 51 52 45 52 52 52 102 45 98 51 100 100 45 49 49 55 56 55 51 98 48 99 100 101 102 34 44 34 108 101 97 115 101 68 117 114 97 116 105 111 110 83 101 99 111 110 100 115 34 58 49 53 44 34 97 99 113 117 105 114 101 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 49 58 53 53 58 53 54 46 48 48 48 48 48 48 90 34 44 34 114 101 110 101 119 84 105 109 101 34 58 34 50 48 50 48 45 49 50 45 50 50 84 50 50 58 48 49 58 52 57 46 56 57 50 53 48 57 90 34 44 34 108 101 97 115 101 84 114 97 110 115 105 116 105 111 110 115 34 58 53 52 125 125] <nil>},DryRun:*false,Options:{[123 34 107 105 110 100 34 58 34 85 112 100 97 116 101 79 112 116 105 111 110 115 34 44 34 97 112 105 86 101 114 115 105 111 110 34 58 34 109 101 116 97 46 107 56 115 46 105 111 47 118 49 34 125] <nil>},RequestKind:coordination.k8s.io/v1, Kind=Lease,RequestResource:coordination.k8s.io/v1, Resource=leases,RequestSubResource:,},Response:nil,}
I1222 22:01:52.048094       1 main.go:93] [serve] Schema GroupVersionKind:
admission.k8s.io/v1beta1, Kind=AdmissionReview
I1222 22:01:52.048532       1 main.go:104] [serve] Constructing response
```
