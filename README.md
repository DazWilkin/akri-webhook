# Akri: ValidatingAdmissionWebhook for Configurations (CRD)

See: https://github.com/deislabs/akri/issues/180 Specifically: https://github.com/deislabs/akri/issues/180#issuecomment-748540637

References:

+ Kubernetes [A Guide to Kubernetes Admission Controllers](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
+ Kubernetes [Admission Controllers: ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook)
+ Kubernetes E2E tests [webhook](https://github.com/kubernetes/kubernetes/blob/v1.13.0/test/images/webhook/main.go)
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
SERVICE="wednesday"
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

${SERVICE}.${NAMESPACE}.crt
${SERVICE}.${NAMESPACE}.csr
${SERVICE}.${NAMESPACE}.key
```

And:

```bash
kubectl get validatingwebhookconfiguration/${SERVICE}
```


## Deploy

But:

```bash
# Deploy Webhook
cat ./webhook.deployment.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=-

# Expose Webhook (Deployment)
cat ./webhook.service.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| kubectl apply --filename=-

# Configurae K8s to use the Webhook
cat ./validatingwebhook.yaml \
| sed "s|SERVICE|${SERVICE}|g" \
| sed "s|NAMESPACE|${NAMESPACE}|g" \
| sed "s|CABUNDLE|${CA_BUNDLE}|g" \
| kubectl apply --filename=-
```

## Verify

```bash
kubectl get deployment --selector=project=akri,component=webhook
kubectl get pod --selector=project=akri,component=webhook
kubectl get service --selector=project=akri,component=webhook
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
