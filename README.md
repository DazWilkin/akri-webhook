# Akri: ValidatingAdmissionWebhook for Configurations (CRD)

See: https://github.com/deislabs/akri/issues/180 Specifically: https://github.com/deislabs/akri/issues/180#issuecomment-748540637

References:

+ Kubernetes [A Guide to Kubernetes Admission Controllers](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
+ Kubernetes [Admission Controllers: ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook)
+ Kubernetes E2E tests [webhook](https://github.com/kubernetes/kubernetes/blob/v1.13.0/test/images/webhook/main.go)
+ Kubernetes API Reference [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#validatingwebhookconfiguration-v1-admissionregistration-k8s-io)

## Build

```bash
REPO="ghcr.io/dazwilkin/akri-webhook"
TAGS=$(git rev-parse HEAD)

docker build \
--tag=${REPO}:${TAGS} \
--file=./Dockerfile \
.
```

## Local Testing

### Certificate

```bash
openssl req \
-x509 \
-newkey rsa:2048 \
-keyout ./secrets/localhost.key \
-out ./secrets/localhost.crt \
-nodes \
-days 365 \
-subj "/CN=localhost"
```

### Run

Either:

```bash
go run . \
--tls-crt-file=./secrets/localhost.crt \
--tls-key-file=./secrets/localhost.key \
--port=8443 \
--logtostderr --v=2
```


```bash
REPO="ghcr.io/dazwilkin/akri-webhook"
TAGS=$(git rev-parse HEAD)

docker run \
--rm --interactive --tty \
--publish=8443:8443 \
--volume=${PWD}/secrets:/secrets \
${REPO}:${TAGS} \
  --tls-crt-file=/secrets/localhost.crt \
  --tls-key-file=/secrets/localhost.key \
  --port=8443 \
  --logtostderr --v=2
```

Then, from another shell:

```bash
VERS="v1" # Version of `admissionregistration.k8s.io`

for TEST in "good" "bad"
do
  RESP=$(curl \
  --silent \
  --insecure \
  --cert ./secrets/localhost.crt \
  --key ./secrets/localhost.key \
  --request POST \
  --header "Content-Type: application/json" \
  --data "@./JSON/admissionreview.${VERS}.rqst.${TEST}.json" \
  https://hades-canyon.local:8443/validate)
  printf "${TEST}: ${RESP}\n"
done
```

> **NOTE** you may add `--write-out '%{response_code}'` to check the response code

Yields:

```console
good: {"response":{"uid":"2b752327-a529-4ffd-b2e2-478455e80a0d","allowed":true,"status":{"metadata":{}}}}
bad: {"response":{"uid":"2b752327-a529-4ffd-b2e2-478455e80a0d","allowed":false,"status":{"metadata":{},"message":"Configuration does not include `{.spec.brokerPodSpec.containers[*].resources.limits}[{{PLACEHOLDER}}]`"}}}
```

## Kubernetes

### Certificate

```bash
DIR=${PWD}/secrets
SERVICE="thursday"
NAMESPACE="default"

FILENAME="${DIR}/${SERVICE}.${NAMESPACE}"

openssl req \
-new \
-sha256 \
-newkey rsa:2048 \
-keyout ${FILENAME}.key \
-out ${FILENAME}.csr \
-nodes \
-subj "/CN=${SERVICE}.${NAMESPACE}.svc"

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
```

Yields:

```bash
ls -la secrets

${SERVICE}.${NAMESPACE}.crt
${SERVICE}.${NAMESPACE}.csr
${SERVICE}.${NAMESPACE}.key
```

### Deploy

But:

```bash
# Deploy Webhook
cat ./kubernetes/deployment.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=- --namespace=${NAMESPACE}

# Expose Webhook (Deployment)
cat ./kubernetes/service.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=- --namespace=${NAMESPACE}

CABUNDLE=$(\
  kubectl get secrets \
  --namespace=${NAMESPACE} \
  --output=jsonpath="{.items[?(@.metadata.annotations['kubernetes\.io/service-account\.name']=='default')].data.ca\.crt}"\
) && echo ${CABUNDLE}

# Configure K8s to use the Webhook
cat ./kubernetes/webhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|${CABUNDLE}|g" \
| kubectl apply --filename=- --namespace=${NAMESPACE}
```

### Verify

```bash
kubectl get deployment/${SERVICE} --namespace=${NAMESPACE}
kubectl get service/${SERVICE} --namespace=${NAMESPACE}
kubectl get validatingwebhookconfiguration/${SERVICE} --namespace=${NAMESPACE}
```

And:

```bash
kubectl logs deployment/${SERVICE} --namespace=${NAMESPACE}
```

Should yield:

```console
[main] Loading key-pair [/secrets/tls.crt, /secrets/tls.key]
[main] Starting Server [:8443]
```

> **NOTE** The `Deployment` runs the webhook container on port `:8443` (shown above) but the `Service` maps this to `:443` and the `ValidatingWebhookConfiguration` is configured to use the `Service` on `:443`.

### Test

In order to test the Webhook, we need to create an `akri.sh/v0/Configuration` (CRD). You can do this by deploying any Akri Configuration, perhaps:

```bash
kubectl apply --filename=./zeroconf.yaml
```

Because `zeroconf.yaml` is an `akri.sh/v0/Configuration` its creation or update will trigger the webhook.

```console
[main] Loading key-pair [/secrets/tls.crt, /secrets/tls.key]
[main] Starting Server [:8443]
[serve] Entering
[serve] Method: POST
[serve] Body: { ... "kind":{"group":"akri.sh","version":"v0","kind":"Configuration"} ... }
[serve] Request: {TypeMeta:{Kind:AdmissionReview ... }
[serve] Response: AdmissionResponse{ ... }
```

But, if we mangle `zeroconf.yaml` to incorrectly reference `.spec.brokerPodSpec.containers[*].resources.limits`, e.g.:

```YAML
apiVersion: akri.sh/v0
kind: Configuration
metadata:
  name: zeroconf
spec:
  protocol:
    zeroconf:
      kind: "_rust._tcp"
      port: 8888
      txtRecords:
        project: akri
        protocol: zeroconf
        component: avahi-publish
  capacity: 1
  brokerPodSpec:
    imagePullSecrets: # Container Registry secret
      - name: ghcr
    containers:
      - name: zeroconf-broker
        image: ghcr.io/dazwilkin/zeroconf-broker@sha256:69810b622d37d0a9a544955d4d4c53f16fec6b8d32a111740f4503dcc164fcf0
  resources: <------ INCORRECTLY INDENTED SO IT DOES NOT APPlY TO `containers`
    limits:
      "{{PLACEHOLDER}}": "1"
```

And apply it:

```bash
kubectl apply --filename=./zeroconf.yaml 
Error from server: error when creating "./zeroconf.yaml": admission webhook denied the request
Configuration does not include `{.spec.brokerPodSpec.containers[*].resources.limits}[{{PLACEHOLDER}}]`
```

> **NOTE** I've edited the error message to make it easier to read here. The key message is that the Configuration does not include the expected `resources` section, because we intentionally broke the YAML.

### Deleting

```bash
cat ./webhook.deployment.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl delete --filename=- --namespace=${NAMESPACE}

cat ./webhook.service.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl delete --filename=- --namespace=${NAMESPACE}

cat ./validatingwebhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|${CA_BUNDLE}|g" \
| kubectl delete --filename=- --namespace=${NAMESPACE}
```

Or, more succinctly:

```bash
kubectl delete deployment/${SERVICE} \
--namespace=${NAMESPACE}

kubectl delete service/${SERVICE} \
--namespace=${NAMESPACE}

kubectl delete validatingwebhookconfiguration/${SERVICE} \
--namespace=${NAMESPACE}

kubectl delete secret/${SERVICE} \
--namespace=${NAMESPACE}
```

Or even more succintly if you used a non-default namespace:

```bash
kubectl delete namespace/${NAMESPACE}
```

> **NOTE** You'll receive `warning: deleting cluster-scoped resources, not scoped to the provided namespace` because the `ValidatingWebhookConfiguration` although created in `${NAMESPACE}` applies to `akri.sh/v0/Configuration` created in any namespace.

You may also want to tidy any remaining CSRs if you're confident you won't need them:

```bash
kubectl delete csr/${SERVICE}.${NAMESPACE}
```

## Debugging

Using Zeroconf, need some services published for the Akri Agent to find:

```bash
KIND="_rust._tcp"
PORT="8888"
TXT_RECORDS=("project=akri" "protocol=zeroconf" "component=avahi-publish")

for SERVICE in "mars" "venus" "jupiter" "saturn" "neptune" "uranus"
do
  avahi-publish --service ${SERVICE} ${KIND} ${PORT} ${TXT_RECORDS[@]} & 
done
```

Then:

```bash
REPO="ghcr.io/dazwilkin"
VERS="v0.0.44-amd64"

sudo microk8s.helm3 install akri ./akri/deployment/helm \
--set imagePullSecrets[0].name=ghcr \
--set agent.image.repository=${REPO}/agent \
--set agent.image.tag=${VERS} \
--set controller.image.repository=${REPO}/controller \
--set controller.image.tag=${VERS}

kubectl apply --filename=./zeroconf.yaml
```

Then `curl` Webhook's `/validate` endpoint:

```bash
kubectl run curl --stdin --tty --rm --image=curlimages/curl -- sh
curl \
--insecure \
--request POST \
--header "Content-Type: application/json:" \
https://${SERVICE}.${NAMESPACE}.svc/validate
```

Then check the deployment's logs:

```bash
kubectl logs  --selector=project=akri
[serve] Entering
[serve] Method: POST
[serve] Handling request:
[serve] Request: {TypeMeta:{Kind: APIVersion:} Request:nil Response:nil}
[serve] Runtime Object: &AdmissionReview{Request:nil,Response:nil,}
[serve] Schema GroupVersionKind: /, Kind=
E1223 18:08:50.741224       1 main.go:96] Admission Review request is nil
```

And:

```YAML
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1beta1",
  "request": {
    "uid": "982d399b-d3f0-42b9-8ca3-4a6dc75e09e6",
    "kind": {
      "group": "akri.sh",
      "version": "v0",
      "kind": "Configuration"
    },
    "resource": {
      "group": "akri.sh",
      "version": "v0",
      "resource": "configurations"
    },
    "requestKind": {
      "group": "akri.sh",
      "version": "v0",
      "kind": "Configuration"
    },
    "requestResource": {
      "group": "akri.sh",
      "version": "v0",
      "resource": "configurations"
    },
    "name": "zeroconf",
    "namespace": "default",
    "operation": "CREATE",
    "userInfo": {
      "username": "admin",
      "uid": "admin",
      "groups": ["system:masters", "system:authenticated"]
    },
    "object": {
      "apiVersion": "akri.sh/v0",
      "kind": "Configuration",
      "metadata": {
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"akri.sh/v0\",\"kind\":\"Configuration\",\"metadata\":{\"annotations\":{},\"name\":\"zeroconf\",\"namespace\":\"default\"},\"spec\":{\"brokerPodSpec\":{\"containers\":[{\"image\":\"ghcr.io/dazwilkin/zeroconf-broker@sha256:69810b622d37d0a9a544955d4d4c53f16fec6b8d32a111740f4503dcc164fcf0\",\"name\":\"zeroconf-broker\",\"resources\":{\"limits\":{\"{{PLACEHOLDER}}\":\"1\"}}}],\"imagePullSecrets\":[{\"name\":\"ghcr\"}]},\"capacity\":1,\"protocol\":{\"zeroconf\":{\"kind\":\"_rust._tcp\",\"port\":8888,\"txtRecords\":{\"component\":\"avahi-publish\",\"project\":\"akri\",\"protocol\":\"zeroconf\"}}}}}\n"
        },
        "creationTimestamp": "2020-12-23T20:20:43Z",
        "generation": 1,
        "managedFields": [{
          "apiVersion": "akri.sh/v0",
          "fieldsType": "FieldsV1",
          "fieldsV1": {
            "f:metadata": {
              "f:annotations": {
                ".": {},
                "f:kubectl.kubernetes.io/last-applied-configuration": {}
              }
            },
            "f:spec": {
              ".": {},
              "f:brokerPodSpec": {
                ".": {},
                "f:containers": {},
                "f:imagePullSecrets": {}
              },
              "f:capacity": {},
              "f:protocol": {
                ".": {},
                "f:zeroconf": {
                  ".": {},
                  "f:kind": {},
                  "f:port": {},
                  "f:txtRecords": {
                    ".": {},
                    "f:component": {},
                    "f:project": {},
                    "f:protocol": {}
                  }
                }
              }
            }
          },
          "manager": "kubectl",
          "operation": "Update",
          "time": "2020-12-23T20:20:43Z"
        }],
        "name": "zeroconf",
        "namespace": "default",
        "uid": "8a8b372f-a301-4afb-9603-b9a6f9573c2d"
      },
      "spec": {
        "brokerPodSpec": {
          "containers": [{
            "image": "ghcr.io/dazwilkin/zeroconf-broker@sha256:69810b622d37d0a9a544955d4d4c53f16fec6b8d32a111740f4503dcc164fcf0",
            "name": "zeroconf-broker",
            "resources": {
              "limits": {
                "{{PLACEHOLDER}}": "1"
              }
            }
          }],
          "imagePullSecrets": [{
            "name": "ghcr"
          }]
        },
        "capacity": 1,
        "protocol": {
          "zeroconf": {
            "kind": "_rust._tcp",
            "port": 8888,
            "txtRecords": {
              "component": "avahi-publish",
              "project": "akri",
              "protocol": "zeroconf"
            }
          }
        }
      }
    },
    "oldObject": null,
    "dryRun": false,
    "options": {
      "kind": "CreateOptions",
      "apiVersion": "meta.k8s.io/v1"
    }
  }
}
```

And:

```YAML
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1beta1",
  "request": {
    "uid": "13b3ee37-7d5e-467e-a96c-75d5ca67ae62",
    "kind": {
      "group": "akri.sh",
      "version": "v0",
      "kind": "Instance"
    },
    "resource": {
      "group": "akri.sh",
      "version": "v0",
      "resource": "instances"
    },
    "requestKind": {
      "group": "akri.sh",
      "version": "v0",
      "kind": "Instance"
    },
    "requestResource": {
      "group": "akri.sh",
      "version": "v0",
      "resource": "instances"
    },
    "name": "zeroconf-ef5d4a",
    "namespace": "default",
    "operation": "CREATE",
    "userInfo": {
      "username": "system:serviceaccount:default:akri-agent-sa",
      "uid": "2693aee6-8755-4555-8cf2-dad8a6ac67f6",
      "groups": ["system:serviceaccounts", "system:serviceaccounts:default", "system:authenticated"]
    },
    "object": {
      "apiVersion": "akri.sh/v0",
      "kind": "Instance",
      "metadata": {
        "creationTimestamp": "2020-12-23T20:20:53Z",
        "generation": 1,
        "managedFields": [{
          "apiVersion": "akri.sh/v0",
          "fieldsType": "FieldsV1",
          "fieldsV1": {
            "f:metadata": {
              "f:ownerReferences": {
                ".": {},
                "k:{\"uid\":\"8a8b372f-a301-4afb-9603-b9a6f9573c2d\"}": {
                  ".": {},
                  "f:apiVersion": {},
                  "f:blockOwnerDeletion": {},
                  "f:controller": {},
                  "f:kind": {},
                  "f:name": {},
                  "f:uid": {}
                }
              }
            },
            "f:spec": {
              ".": {},
              "f:configurationName": {},
              "f:deviceUsage": {
                ".": {},
                "f:zeroconf-ef5d4a-0": {}
              },
              "f:metadata": {
                ".": {},
                "f:AKRI_ZEROCONF": {},
                "f:AKRI_ZEROCONF_DEVICE_ADDR": {},
                "f:AKRI_ZEROCONF_DEVICE_COMPONENT": {},
                "f:AKRI_ZEROCONF_DEVICE_HOST": {},
                "f:AKRI_ZEROCONF_DEVICE_KIND": {},
                "f:AKRI_ZEROCONF_DEVICE_NAME": {},
                "f:AKRI_ZEROCONF_DEVICE_PORT": {},
                "f:AKRI_ZEROCONF_DEVICE_PROJECT": {},
                "f:AKRI_ZEROCONF_DEVICE_PROTOCOL": {}
              },
              "f:nodes": {},
              "f:rbac": {},
              "f:shared": {}
            }
          },
          "manager": "unknown",
          "operation": "Update",
          "time": "2020-12-23T20:20:53Z"
        }],
        "name": "zeroconf-ef5d4a",
        "namespace": "default",
        "ownerReferences": [{
          "apiVersion": "akri.sh/v0",
          "blockOwnerDeletion": true,
          "controller": true,
          "kind": "Configuration",
          "name": "zeroconf",
          "uid": "8a8b372f-a301-4afb-9603-b9a6f9573c2d"
        }],
        "uid": "367edab0-8df7-4212-ab43-eb93560bb8d2"
      },
      "spec": {
        "configurationName": "zeroconf",
        "deviceUsage": {
          "zeroconf-ef5d4a-0": ""
        },
        "metadata": {
          "AKRI_ZEROCONF": "zeroconf",
          "AKRI_ZEROCONF_DEVICE_ADDR": "10.138.0.2",
          "AKRI_ZEROCONF_DEVICE_COMPONENT": "avahi-publish",
          "AKRI_ZEROCONF_DEVICE_HOST": "akri.local",
          "AKRI_ZEROCONF_DEVICE_KIND": "_rust._tcp",
          "AKRI_ZEROCONF_DEVICE_NAME": "freddie",
          "AKRI_ZEROCONF_DEVICE_PORT": "8888",
          "AKRI_ZEROCONF_DEVICE_PROJECT": "akri",
          "AKRI_ZEROCONF_DEVICE_PROTOCOL": "zeroconf"
        },
        "nodes": ["akri"],
        "rbac": "rbac",
        "shared": true
      }
    },
    "oldObject": null,
    "dryRun": false,
    "options": {
      "kind": "CreateOptions",
      "apiVersion": "meta.k8s.io/v1"
    }
  }
}
```
