/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gce

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	gce "google.golang.org/api/compute/v1"
	gke "google.golang.org/api/container/v1"
)

const (
	projectId   = "project1"
	zone        = "us-central1-b"
	clusterName = "cluster1"
)

type httpServerMock struct {
	mock.Mock
	*httptest.Server
}

func NewHttpServerMock() *httpServerMock {
	httpServerMock := &httpServerMock{}
	mux := http.NewServeMux()
	mux.HandleFunc("/",
		func(w http.ResponseWriter, req *http.Request) {
			result := httpServerMock.handle(req.URL.Path)
			w.Write([]byte(result))
		})

	server := httptest.NewServer(mux)
	httpServerMock.Server = server
	return httpServerMock
}

func (l *httpServerMock) handle(url string) string {
	glog.Warning("Handle %v", url)
	args := l.Called(url)
	return args.String(0)
}

const allNodePools1 = `{
  "nodePools": [
    {
      "name": "default-pool",
      "config": {
        "machineType": "n1-standard-1",
        "diskSizeGb": 100,
        "oauthScopes": [
          "https://www.googleapis.com/auth/compute",
          "https://www.googleapis.com/auth/devstorage.read_only",
          "https://www.googleapis.com/auth/logging.write",
          "https://www.googleapis.com/auth/monitoring.write",
          "https://www.googleapis.com/auth/servicecontrol",
          "https://www.googleapis.com/auth/service.management.readonly",
          "https://www.googleapis.com/auth/trace.append"
        ],
        "imageType": "COS",
        "serviceAccount": "default"
      },
      "initialNodeCount": 3,
      "autoscaling": {"Enabled": true},
      "management": {},
      "selfLink": "https://container.googleapis.com/v1/projects/project1/zones/us-central1-a/clusters/cluster-1/nodePools/default-pool",
      "version": "1.6.9",
      "instanceGroupUrls": [
        "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool"
      ],
      "status": "RUNNING"
    }
  ]
}`

const allNodePools2 = `{
  "nodePools": [
    {
      "name": "default-pool",
      "config": {
        "machineType": "n1-standard-1",
        "diskSizeGb": 100,
        "oauthScopes": [
          "https://www.googleapis.com/auth/compute",
          "https://www.googleapis.com/auth/devstorage.read_only",
          "https://www.googleapis.com/auth/logging.write",
          "https://www.googleapis.com/auth/monitoring.write",
          "https://www.googleapis.com/auth/servicecontrol",
          "https://www.googleapis.com/auth/service.management.readonly",
          "https://www.googleapis.com/auth/trace.append"
        ],
        "imageType": "COS",
        "serviceAccount": "default"
      },
      "initialNodeCount": 3,
      "autoscaling": {"Enabled": true},
      "management": {},
      "selfLink": "https://container.googleapis.com/v1/projects/project1/zones/us-central1-a/clusters/cluster-1/nodePools/default-pool",
      "version": "1.6.9",
      "instanceGroupUrls": [
        "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool"
      ],
      "status": "RUNNING"
    },
    {
      "name": "default-pool",
      "config": {
        "machineType": "n1-standard-1",
        "diskSizeGb": 100,
        "oauthScopes": [
          "https://www.googleapis.com/auth/compute",
          "https://www.googleapis.com/auth/devstorage.read_only",
          "https://www.googleapis.com/auth/logging.write",
          "https://www.googleapis.com/auth/monitoring.write",
          "https://www.googleapis.com/auth/servicecontrol",
          "https://www.googleapis.com/auth/service.management.readonly",
          "https://www.googleapis.com/auth/trace.append"
        ],
        "imageType": "COS",
        "serviceAccount": "default"
      },
      "initialNodeCount": 3,
      "autoscaling": {"Enabled": true},
      "management": {},
      "selfLink": "https://container.googleapis.com/v1/projects/project1/zones/us-central1-a/clusters/cluster-1/nodePools/node_pool2",
      "version": "1.6.9",
      "instanceGroupUrls": [
        "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-node_pool2"
      ],
      "status": "RUNNING"
    },
    {
      "name": "default-pool",
      "config": {
        "machineType": "n1-standard-1",
        "diskSizeGb": 100,
        "oauthScopes": [
          "https://www.googleapis.com/auth/compute",
          "https://www.googleapis.com/auth/devstorage.read_only",
          "https://www.googleapis.com/auth/logging.write",
          "https://www.googleapis.com/auth/monitoring.write",
          "https://www.googleapis.com/auth/servicecontrol",
          "https://www.googleapis.com/auth/service.management.readonly",
          "https://www.googleapis.com/auth/trace.append"
        ],
        "imageType": "COS",
        "serviceAccount": "default"
      },
      "initialNodeCount": 3,
      "autoscaling": {},
      "management": {},
      "selfLink": "https://container.googleapis.com/v1/projects/project1/zones/us-central1-a/clusters/cluster-1/nodePools/node_pool3",
      "version": "1.6.9",
      "instanceGroupUrls": [
        "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-node_pool3"
      ],
      "status": "RUNNING"
    }
  ]
}`

const instanceGroupManager = `{
 "kind": "compute#instanceGroupManager",
 "id": "3213213219",
 "creationTimestamp": "2017-09-15T04:47:24.687-07:00",
 "name": "gke-cluster-1-default-pool",
 "zone": "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a",
 "instanceTemplate": "https://www.googleapis.com/compute/v1/projects/project1/global/instanceTemplates/gke-cluster-1-default-pool",
 "instanceGroup": "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroups/gke-cluster-1-default-pool",
 "baseInstanceName": "gke-cluster-1-default-pool-f23aac-grp",
 "fingerprint": "kfdsuH",
 "currentActions": {
  "none": 3,
  "creating": 0,
  "creatingWithoutRetries": 0,
  "recreating": 0,
  "deleting": 0,
  "abandoning": 0,
  "restarting": 0,
  "refreshing": 0
 },
 "targetSize": 3,
 "selfLink": "https://www.googleapis.com/compute/v1/projects/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool"
}
`
const instanceTemplate = `
{
 "kind": "compute#instanceTemplate",
 "id": "28701103232323232",
 "creationTimestamp": "2017-09-15T04:47:21.577-07:00",
 "name": "gke-cluster-1-default-pool",
 "description": "",
 "properties": {
  "tags": {
   "items": [
    "gke-cluster-1-fc0afeeb-node"
   ]
  },
  "machineType": "n1-standard-1",
  "canIpForward": true,
  "networkInterfaces": [
   {
    "kind": "compute#networkInterface",
    "network": "https://www.googleapis.com/compute/v1/projects/krzysztof-jastrzebski-dev/global/networks/default",
    "subnetwork": "https://www.googleapis.com/compute/v1/projects/krzysztof-jastrzebski-dev/regions/us-central1/subnetworks/default",
    "accessConfigs": [
     {
      "kind": "compute#accessConfig",
      "type": "ONE_TO_ONE_NAT",
      "name": "external-nat"
     }
    ]
   }
  ],
  "disks": [
   {
    "kind": "compute#attachedDisk",
    "type": "PERSISTENT",
    "mode": "READ_WRITE",
    "boot": true,
    "initializeParams": {
     "sourceImage": "https://www.googleapis.com/compute/v1/projects/gke-node-images/global/images/cos-stable-60-9592-84-0",
     "diskSizeGb": "100",
     "diskType": "pd-standard"
    },
    "autoDelete": true
   }
  ],
  "metadata": {
   "kind": "compute#metadata",
   "fingerprint": "F7n_RsHD3ng=",
   "items": [
    {
     "key": "kube-env",
     "value": "ALLOCATE_NODE_CIDRS: \"true\"\nCA_CERT: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURERENDQWZTZ0F3SUJBZ0lSQU14MVpPRElzYjRGTmVXRjlMNjZHRTB3RFFZSktvWklodmNOQVFFTEJRQXcKTHpFdE1Dc0dBMVVFQXhNa1pUUXlNVGt5TkdRdE1HSXlOaTAwWVdKakxUZ3hNemN0WWpKak1HTTNZMlJsTmprNApNQjRYRFRFM01Ea3hOVEV3TkRjd01sb1hEVEl5TURreE5ERXhORGN3TWxvd0x6RXRNQ3NHQTFVRUF4TWtaVFF5Ck1Ua3lOR1F0TUdJeU5pMDBZV0pqTFRneE16Y3RZakpqTUdNM1kyUmxOams0TUlJQklqQU5CZ2txaGtpRzl3MEIKQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBdEdPaFl3Rzk4TU02b01tQ0xWQjgzU2ZtanIwMUdwZXZsQ3FkTk5CQwpvQlZJcS9YdmloSmoreEJZMlg3ajRFN3MyUWtnWEdWQ3Bpb2IvVERodEtUb0FkT0NGZXV1SlZvUTRQdEVwK2xkCnNvdElDU3BJWFZtSHZPSWR3SmdubndLU0xBMGk3LytML3NmQ3I5ZVBoOUhHaThQUk9NTnhteFlmWDJpRy9xV20KUlAyZHJXOFM4Rk9Ud2JvRzFvOTJnNC9iZTcvN0xEeTRXOGh6VjNZYVBxVXhnZFZBYXVWRGMxTlhDaHJQbTlzTwo3MElHUkhmVXQwSzQvVEE0ZHFMbjdzN0ErQVBRcVIxby9oOVIxU0ZKN2k2a1JzMjViL2kxeGpzNkNWWittYTQ2CmNKVFRPeEFmTkdQRXdiL3BrS1hlRnNNUDR2ZlJFRUJ5NHUzUWZtNzVHcmNma1FJREFRQUJveU13SVRBT0JnTlYKSFE4QkFmOEVCQU1DQWdRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQQpuSk9WcVNCWXkwNTFjQTVLVDNHODVVL1JOYzlFREEydlo5U0lmK1F4N213TnpIeFBWRmE4bmtReHAzem1oMmlnCk94NTNhTDZlMnErNnJOWW8xNzloc1I3YzRCSnVxZXMrVHBHT3R5eXBuY2gzNncwY25hdWpUR1RFaXdjWFphYUQKb2hlaXl4SU5Ud3p0Q0ZzRkNydzRNSTdWSEY1ektCOVE5UWNBYnJNVkxQVHVIdUI3V29aOXdUa1cvRGRldEJScApEWjhqVUp5elBvdUFFZTZQR2pPL005dEZSRSsvYUNJMERMN2ZnU3dzNXA4OTZhNFZkNDYzcFhmZWVIOGcvV3dBCkhMdlVxWFhNSDg4dEhrd2ZNWFVZTkJWT2JsN21ua1E5OFUwOG5aZnJ0TWthc0lYc1JTY0h4S3VIbko4QnZCYlYKTHlRQzErNk81WWRVYU1DcWIzS2FWZz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\nCLUSTER_IP_RANGE: 10.8.0.0/14\nCLUSTER_NAME: cluster-1\nDNS_DOMAIN: cluster.local\nDNS_SERVER_IP: 10.11.240.10\nELASTICSEARCH_LOGGING_REPLICAS: \"1\"\nENABLE_CLUSTER_DNS: \"true\"\nENABLE_CLUSTER_LOGGING: \"false\"\nENABLE_CLUSTER_MONITORING: standalone\nENABLE_CLUSTER_REGISTRY: \"false\"\nENABLE_CLUSTER_UI: \"true\"\nENABLE_L7_LOADBALANCING: glbc\nENABLE_NODE_LOGGING: \"true\"\nENABLE_NODE_PROBLEM_DETECTOR: standalone\nENV_TIMESTAMP: 2017-09-15T11:47:02+00:00\nEVICTION_HARD: memory.available\u003c100Mi,nodefs.available\u003c10%,nodefs.inodesFree\u003c5%\nEXTRA_DOCKER_OPTS: --insecure-registry 10.0.0.0/8\nFEATURE_GATES: ExperimentalCriticalPodAnnotation=true\nINSTANCE_PREFIX: gke-cluster-1-fc0afeeb\nKUBE_ADDON_REGISTRY: gcr.io/google_containers\nKUBE_MANIFESTS_TAR_HASH: 0c2eb72b9a88503e9faf02d276ab774b1749c1d9\nKUBE_MANIFESTS_TAR_URL: https://storage.googleapis.com/kubernetes-release/release/v1.6.9/kubernetes-manifests.tar.gz,https://storage.googleapis.com/kubernetes-release-eu/release/v1.6.9/kubernetes-manifests.tar.gz,https://storage.googleapis.com/kubernetes-release-asia/release/v1.6.9/kubernetes-manifests.tar.gz\nKUBE_PROXY_TOKEN: Al8upGNIELfIcuF1E-oUA5S4RVrr90_WJsoLUd371zw=\nKUBELET_CERT: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMzRENDQWNTZ0F3SUJBZ0lSQVBHOHk1TGFTSlNlTEJ3bGtWMjNCR2t3RFFZSktvWklodmNOQVFFTEJRQXcKTHpFdE1Dc0dBMVVFQXhNa1pUUXlNVGt5TkdRdE1HSXlOaTAwWVdKakxUZ3hNemN0WWpKak1HTTNZMlJsTmprNApNQjRYRFRFM01Ea3hOVEV4TkRjd05Gb1hEVEl5TURreE5ERXhORGN3TkZvd0VqRVFNQTRHQTFVRUF4TUhhM1ZpClpXeGxkRENDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQkFMdUZVK0NxcDc3dzF2QlQKUlg3YTJjVjBEY0JsQVM1ZmxUYnUyRHJDVzlEbWNxb28yQ3hvRGt3SDNtcjkzMmhvc0Q0eEI0YkZwbzZlN0g3bQpHYkQzNkNtOERURXZZMmU4ei9Wem5SUXc2cjJNdHRGd0E2NXJMZDR3TkFiRkpGb053cE5RZjBzU240Y0JERFE5ClJydkhuS28zT3RNaUJSVzd0TnpkajhsT1E3UkVqbVV2aHpiYm5RNTNqM1pBakJhMzM4TERqdXhQVzE5ZXJTc1YKRHZoWHlFYjhVTVltREdTZCs0R2RhaU9IUndFZmZJTndUZXdNUUlBS2hsNnRleUwrb0U5Zkd2bGRMaXdkN0tpNAo1aXFZbW1kQ2N3STk4cE1DMy90NnJxTS9UQVVmbVVkb0ZCbld5a1R6RnJQTmVSdXpEYW1MbVFrUzRxS1ExMlpYClkvWHUvdDBDQXdFQUFhTVFNQTR3REFZRFZSMFRBUUgvQkFJd0FEQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUEKR210UjZPbWFRWkRBNjB6M09MRjhTSzB2eCt5Qzl5SmZidFFXQU11ZCs2dVIzUWFhSkdDb1lRdEZhYjJqRWt3Zgp2Q0VWcTJXWWZZeXRlMHJ2eUNxNVI4MGR6R1FlMzlHUmFNcFN3R2t1MjgwYTIxTWlaQWJ0ZmgwSCtmeDRzN2RnCmQ0OG5Ja0NqTHdDWW55dG5ua0ticVdMUXVtVGdQaUNPRWN6eGM4VTlZd0RlWTlrZ3Q1MFNxRzRLNlFvMjRWaXYKTFAyM1ByQllUWHB3dlk3aUJjTzg2OWxZZ2R0Ni9FV3Y4NGVMdEc0VUFRamJpVnNsYnVuak5KcHRTWDJNY3VoTApqaHUzMEVOeVZBMVAwVVV1RVpGcXhRSU5vS1pwNlJKSlBHc0FwYWNibmtSV2hZd2RLNGRoL3EzL2Raei9HNFd1Ck5hYkRMakxvaWU4RURCT0V6RWFweGc9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==\nKUBELET_KEY: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBdTRWVDRLcW52dkRXOEZORmZ0clp4WFFOd0dVQkxsK1ZOdTdZT3NKYjBPWnlxaWpZCkxHZ09UQWZlYXYzZmFHaXdQakVIaHNXbWpwN3NmdVlac1Bmb0tid05NUzlqWjd6UDlYT2RGRERxdll5MjBYQUQKcm1zdDNqQTBCc1VrV2czQ2sxQi9TeEtmaHdFTU5EMUd1OGVjcWpjNjB5SUZGYnUwM04yUHlVNUR0RVNPWlMrSApOdHVkRG5lUGRrQ01GcmZmd3NPTzdFOWJYMTZ0S3hVTytGZklSdnhReGlZTVpKMzdnWjFxSTRkSEFSOThnM0JOCjdBeEFnQXFHWHExN0l2NmdUMThhK1YwdUxCM3NxTGptS3BpYVowSnpBajN5a3dMZiszcXVvejlNQlIrWlIyZ1UKR2RiS1JQTVdzODE1RzdNTnFZdVpDUkxpb3BEWFpsZGo5ZTcrM1FJREFRQUJBb0lCQVFDWmRiNHo4VlVBRk5iQgpyRlFHUVpIUVhtNVdraEpLWWh5WjdSTDEyMU1LZlYwL1ZkZTdWNWpEcmZqZWRFN2RnamNoWGp4N2xjcjlrcCtXCkpqYkdOY3RBUkU1RGZ6V0prdUpaUzdrejZlaGhJUVFJRi9SYlRBa29lU2hLbGhGdXhTNEVJdTlaZjY4ZjY4S1MKQ2NlV0t3QlI2SXJ5ZURmVEZsOG9GUVh6eDJIdTVualYzazc2enEwNEMvZVlXdVN5RHd2cWVOZy9NeGtLS0toUgp3aHhObks4ODFqSFpsQTN4dFQ3YWRqNUVmdmZOTHBlNkJkWmlPOGNLU1hJZ0FDMjA4RTA4YnE2RTFjNnZjNm1xCmdmZURIUU5hOFhYU3UvWTJVL3JSUml6bTh0MURvS2ZRNGZvc05GY3FOakxkVE84SU55Y2hwQWpQQ2JCZzcwNk0KUGYyQjF4cWhBb0dCQU9aWGJWWk1Ua3ZhR0duc1I2cm1lN04vcURtM1hudXZCR0EwSitzMFRFdnpvc2t1NmwyMgp1Sml4WGtOM2VlVEwxU3MvVW1YSVg0UHRUOGxnT3kzYkJycmVpdE5ZN0hFZTR1RkNnV2JVa0d6MWtZeVVUUWFRCktkN3hweUdEQWVoc1dCT2MvL2JGT0c5L0RXY2ZzRkkwR3lXL1ZwMXVqVU9hcmJjbTdOckxaUmwxQW9HQkFOQm8KeS9lZmlKaXQ5QWRUOU4vc2V5R2JIYVl3T1M1NmdnZkMwTUlSMVZ6L0tJWmY2czJXTmVxYU1LQ29IM0drMTZsTApmNHpBYjAzeDRXZGhTYjdjK1FId0dna0lsYmt2SmE4WmR6NCthcUI4emNuY2JVd0VlcXZHR3NHNXpiNUxYNE10Ckh3bHpDbFlqOGp2elhsR1JEenZZODN6azk0V0ovdjZJT1VOeDg3ckpBb0dBWUdvbjhmOXVwb0ZieHJxSUpSamsKbm5YSXpKL2NoSmoxays5QTVrcTF4UFR1SnBma3NlVlJ6MWd1eEw2MTN2Y1MrMDgrQml5aERtKysvZU94NGJmVQpVVlBsZUNHNGxvRC9KcHJYMzFzS09SRnhJdzdRVHZiNUQ4REczRmdoN0UrdGJraEJPK0hCaGFvQXlqR1JkRmNyCkpkbTVQNXlPdE1XQ0FTL2g1Sk5PZGlFQ2dZRUFocjlMS1VaUHBnL0ttTFpTdkRrRS81eHdGaFJWMUZSSElFZDQKZkJIVnR2UU91cHJua0pjUE15a1FTYitKM1F0c21Md0VzdmQwdjV1bFZoY1QvRUNaQ3dTM2dLRzVWR3RFWFNzRQo2d3ltR2krM1NrMm5xUis0Ukxtb2NScjJDSlJwSThJWHNCOWVUb1dkUjkrNVd0bUVWUGlYcldmSkZlRThLa3ZmCllsa1o4ZWtDZ1lFQWlnMmhVdGxqK2FSZCtlZHpOeHBCSDA3Y1BJcmRrUWJUTytobExLWFNQSTFuemoycHdvM1UKQVN1dTR5OG12b1JvRVRBVVd1eEh6aHNwRXZNSlY2TE9kTWdyY3pSUjE3TzNwOUhKNEgyenZvYU1GZFhSWjRzbwpKdXIvMmR0a29IemdxbDdnQ0pnaWpCeFhMd3dHVFByQnpzQVI1ZHpMdXNyNGhPVjFPSU5hb21vPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=\nKUBELET_TEST_ARGS: --experimental-allocatable-ignore-eviction\nKUBERNETES_MASTER: \"false\"\nKUBERNETES_MASTER_NAME: 35.188.109.143\nLOGGING_DESTINATION: gcp\nNETWORK_PROVIDER: kubenet\nNODE_LABELS: beta.kubernetes.io/fluentd-ds-ready=true,cloud.google.com/gke-nodepool=default-pool\nNODE_PROBLEM_DETECTOR_TOKEN: zSrNBvlLt3TYDfjIgiGuZNZwetWAIJxJxShGl0PXv3M=\nSALT_TAR_HASH: c645f14fc716cc9a5c752ddc7a021e9ee53c3bd7\nSALT_TAR_URL: https://storage.googleapis.com/kubernetes-release/release/v1.6.9/kubernetes-salt.tar.gz,https://storage.googleapis.com/kubernetes-release-eu/release/v1.6.9/kubernetes-salt.tar.gz,https://storage.googleapis.com/kubernetes-release-asia/release/v1.6.9/kubernetes-salt.tar.gz\nSERVER_BINARY_TAR_HASH: 85f88c723881a092da67230936879909cb7882ac\nSERVER_BINARY_TAR_URL: https://storage.googleapis.com/kubernetes-release/release/v1.6.9/kubernetes-server-linux-amd64.tar.gz,https://storage.googleapis.com/kubernetes-release-eu/release/v1.6.9/kubernetes-server-linux-amd64.tar.gz,https://storage.googleapis.com/kubernetes-release-asia/release/v1.6.9/kubernetes-server-linux-amd64.tar.gz\nSERVICE_CLUSTER_IP_RANGE: 10.11.240.0/20\nZONE: us-central1-a\n"
    },
    {
     "key": "user-data",
     "value": "#cloud-config\n\nwrite_files:\n  - path: /etc/systemd/system/kube-node-installation.service\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Download and install k8s binaries and configurations\n      After=network-online.target\n\n      [Service]\n      Type=oneshot\n      RemainAfterExit=yes\n      ExecStartPre=/bin/mkdir -p /home/kubernetes/bin\n      ExecStartPre=/bin/mount --bind /home/kubernetes/bin /home/kubernetes/bin\n      ExecStartPre=/bin/mount -o remount,exec /home/kubernetes/bin\n      ExecStartPre=/usr/bin/curl --fail --retry 5 --retry-delay 3 --silent --show-error\t-H \"X-Google-Metadata-Request: True\" -o /home/kubernetes/bin/configure.sh http://metadata.google.internal/computeMetadata/v1/instance/attributes/configure-sh\n      ExecStartPre=/bin/chmod 544 /home/kubernetes/bin/configure.sh\n      ExecStart=/home/kubernetes/bin/configure.sh\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kube-node-configuration.service\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Configure kubernetes node\n      After=kube-node-installation.service\n\n      [Service]\n      Type=oneshot\n      RemainAfterExit=yes\n      ExecStartPre=/bin/chmod 544 /home/kubernetes/bin/configure-helper.sh\n      ExecStart=/home/kubernetes/bin/configure-helper.sh\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kube-docker-monitor.service\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Kubernetes health monitoring for docker\n      After=kube-node-configuration.service\n\n      [Service]\n      Restart=always\n      RestartSec=10\n      RemainAfterExit=yes\n      RemainAfterExit=yes\n      ExecStartPre=/bin/chmod 544 /home/kubernetes/bin/health-monitor.sh\n      ExecStart=/home/kubernetes/bin/health-monitor.sh docker\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kubelet-monitor.service\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Kubernetes health monitoring for kubelet\n      After=kube-node-configuration.service\n\n      [Service]\n      Restart=always\n      RestartSec=10\n      RemainAfterExit=yes\n      RemainAfterExit=yes\n      ExecStartPre=/bin/chmod 544 /home/kubernetes/bin/health-monitor.sh\n      ExecStart=/home/kubernetes/bin/health-monitor.sh kubelet\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kube-logrotate.timer\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Hourly kube-logrotate invocation\n\n      [Timer]\n      OnCalendar=hourly\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kube-logrotate.service\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Kubernetes log rotation\n      After=kube-node-configuration.service\n\n      [Service]\n      Type=oneshot\n      ExecStart=-/usr/sbin/logrotate /etc/logrotate.conf\n\n      [Install]\n      WantedBy=kubernetes.target\n\n  - path: /etc/systemd/system/kubernetes.target\n    permissions: 0644\n    owner: root\n    content: |\n      [Unit]\n      Description=Kubernetes\n\n      [Install]\n      WantedBy=multi-user.target\n\nruncmd:\n - systemctl daemon-reload\n - systemctl enable kube-node-installation.service\n - systemctl enable kube-node-configuration.service\n - systemctl enable kube-docker-monitor.service\n - systemctl enable kubelet-monitor.service\n - systemctl enable kube-logrotate.timer\n - systemctl enable kube-logrotate.service\n - systemctl enable kubernetes.target\n - systemctl start kubernetes.target\n"
    },
    {
     "key": "gci-update-strategy",
     "value": "update_disabled"
    },
    {
     "key": "gci-ensure-gke-docker",
     "value": "true"
    },
    {
     "key": "configure-sh",
     "value": "#!/bin/bash\n\n# Copyright 2016 The Kubernetes Authors.\n#\n# Licensed under the Apache License, Version 2.0 (the \"License\");\n# you may not use this file except in compliance with the License.\n# You may obtain a copy of the License at\n#\n#     http://www.apache.org/licenses/LICENSE-2.0\n#\n# Unless required by applicable law or agreed to in writing, software\n# distributed under the License is distributed on an \"AS IS\" BASIS,\n# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n# See the License for the specific language governing permissions and\n# limitations under the License.\n\n# Due to the GCE custom metadata size limit, we split the entire script into two\n# files configure.sh and configure-helper.sh. The functionality of downloading\n# kubernetes configuration, manifests, docker images, and binary files are\n# put in configure.sh, which is uploaded via GCE custom metadata.\n\nset -o errexit\nset -o nounset\nset -o pipefail\n\nfunction set-broken-motd {\n  cat \u003e /etc/motd \u003c\u003cEOF\nBroken (or in progress) Kubernetes node setup! Check the cluster initialization status\nusing the following commands.\n\nMaster instance:\n  - sudo systemctl status kube-master-installation\n  - sudo systemctl status kube-master-configuration\n\nNode instance:\n  - sudo systemctl status kube-node-installation\n  - sudo systemctl status kube-node-configuration\nEOF\n}\n\nfunction download-kube-env {\n  # Fetch kube-env from GCE metadata server.\n  local -r tmp_kube_env=\"/tmp/kube-env.yaml\"\n  curl --fail --retry 5 --retry-delay 3 --silent --show-error \\\n    -H \"X-Google-Metadata-Request: True\" \\\n    -o \"${tmp_kube_env}\" \\\n    http://metadata.google.internal/computeMetadata/v1/instance/attributes/kube-env\n  # Convert the yaml format file into a shell-style file.\n  eval $(python -c '''\nimport pipes,sys,yaml\nfor k,v in yaml.load(sys.stdin).iteritems():\n  print(\"readonly {var}={value}\".format(var = k, value = pipes.quote(str(v))))\n''' \u003c \"${tmp_kube_env}\" \u003e \"${KUBE_HOME}/kube-env\")\n  rm -f \"${tmp_kube_env}\"\n}\n\nfunction download-kube-master-certs {\n  # Fetch kube-env from GCE metadata server.\n  local -r tmp_kube_master_certs=\"/tmp/kube-master-certs.yaml\"\n  curl --fail --retry 5 --retry-delay 3 --silent --show-error \\\n    -H \"X-Google-Metadata-Request: True\" \\\n    -o \"${tmp_kube_master_certs}\" \\\n    http://metadata.google.internal/computeMetadata/v1/instance/attributes/kube-master-certs\n  # Convert the yaml format file into a shell-style file.\n  eval $(python -c '''\nimport pipes,sys,yaml\nfor k,v in yaml.load(sys.stdin).iteritems():\n  print(\"readonly {var}={value}\".format(var = k, value = pipes.quote(str(v))))\n''' \u003c \"${tmp_kube_master_certs}\" \u003e \"${KUBE_HOME}/kube-master-certs\")\n  rm -f \"${tmp_kube_master_certs}\"\n}\n\nfunction validate-hash {\n  local -r file=\"$1\"\n  local -r expected=\"$2\"\n\n  actual=$(sha1sum ${file} | awk '{ print $1 }') || true\n  if [[ \"${actual}\" != \"${expected}\" ]]; then\n    echo \"== ${file} corrupted, sha1 ${actual} doesn't match expected ${expected} ==\"\n    return 1\n  fi\n}\n\n# Retry a download until we get it. Takes a hash and a set of URLs.\n#\n# $1 is the sha1 of the URL. Can be \"\" if the sha1 is unknown.\n# $2+ are the URLs to download.\nfunction download-or-bust {\n  local -r hash=\"$1\"\n  shift 1\n\n  local -r urls=( $* )\n  while true; do\n    for url in \"${urls[@]}\"; do\n      local file=\"${url##*/}\"\n      rm -f \"${file}\"\n      if ! curl -f --ipv4 -Lo \"${file}\" --connect-timeout 20 --max-time 300 --retry 6 --retry-delay 10 \"${url}\"; then\n        echo \"== Failed to download ${url}. Retrying. ==\"\n      elif [[ -n \"${hash}\" ]] && ! validate-hash \"${file}\" \"${hash}\"; then\n        echo \"== Hash validation of ${url} failed. Retrying. ==\"\n      else\n        if [[ -n \"${hash}\" ]]; then\n          echo \"== Downloaded ${url} (SHA1 = ${hash}) ==\"\n        else\n          echo \"== Downloaded ${url} ==\"\n        fi\n        return\n      fi\n    done\n  done\n}\n\nfunction split-commas {\n  echo $1 | tr \",\" \"\\n\"\n}\n\nfunction install-gci-mounter-tools {\n  CONTAINERIZED_MOUNTER_HOME=\"${KUBE_HOME}/containerized_mounter\"\n  mkdir -p \"${CONTAINERIZED_MOUNTER_HOME}\"\n  chmod a+x \"${CONTAINERIZED_MOUNTER_HOME}\"\n  mkdir -p \"${CONTAINERIZED_MOUNTER_HOME}/rootfs\"\n  local -r mounter_tar_sha=\"8003b798cf33c7f91320cd6ee5cec4fa22244571\"\n  download-or-bust \"${mounter_tar_sha}\" \"https://storage.googleapis.com/kubernetes-release/gci-mounter/mounter.tar\"\n  cp \"${dst_dir}/kubernetes/gci-trusty/gci-mounter\" \"${CONTAINERIZED_MOUNTER_HOME}/mounter\"\n  chmod a+x \"${CONTAINERIZED_MOUNTER_HOME}/mounter\"\n  mv \"${KUBE_HOME}/mounter.tar\" /tmp/mounter.tar\n  tar xvf /tmp/mounter.tar -C \"${CONTAINERIZED_MOUNTER_HOME}/rootfs\"\n  rm /tmp/mounter.tar\n  mkdir -p \"${CONTAINERIZED_MOUNTER_HOME}/rootfs/var/lib/kubelet\"\n}\n\n# Install node problem detector binary.\nfunction install-node-problem-detector {\n  if [[ -n \"${NODE_PROBLEM_DETECTOR_VERSION:-}\" ]]; then\n      local -r npd_version=\"${NODE_PROBLEM_DETECTOR_VERSION}\"\n      local -r npd_sha1=\"${NODE_PROBLEM_DETECTOR_TAR_HASH}\"\n  else\n      local -r npd_version=\"v0.3.0\"\n      local -r npd_sha1=\"2e6423c5798e14464271d9c944e56a637ee5a4bc\"\n  fi\n  local -r npd_release_path=\"https://storage.googleapis.com/kubernetes-release\"\n  local -r npd_tar=\"node-problem-detector-${npd_version}.tar.gz\"\n  download-or-bust \"${npd_sha1}\" \"${npd_release_path}/node-problem-detector/${npd_tar}\"\n  local -r npd_dir=\"${KUBE_HOME}/node-problem-detector\"\n  mkdir -p \"${npd_dir}\"\n  tar xzf \"${KUBE_HOME}/${npd_tar}\" -C \"${npd_dir}\" --overwrite\n  mv \"${npd_dir}/bin\"/* \"${KUBE_HOME}/bin\"\n  chmod a+x \"${KUBE_HOME}/bin/node-problem-detector\"\n  rmdir \"${npd_dir}/bin\"\n  rm -f \"${KUBE_HOME}/${npd_tar}\"\n}\n\n# Downloads kubernetes binaries and kube-system manifest tarball, unpacks them,\n# and places them into suitable directories. Files are placed in /home/kubernetes.\nfunction install-kube-binary-config {\n  cd \"${KUBE_HOME}\"\n  local -r server_binary_tar_urls=( $(split-commas \"${SERVER_BINARY_TAR_URL}\") )\n  local -r server_binary_tar=\"${server_binary_tar_urls[0]##*/}\"\n  if [[ -n \"${SERVER_BINARY_TAR_HASH:-}\" ]]; then\n    local -r server_binary_tar_hash=\"${SERVER_BINARY_TAR_HASH}\"\n  else\n    echo \"Downloading binary release sha1 (not found in env)\"\n    download-or-bust \"\" \"${server_binary_tar_urls[@]/.tar.gz/.tar.gz.sha1}\"\n    local -r server_binary_tar_hash=$(cat \"${server_binary_tar}.sha1\")\n  fi\n  echo \"Downloading binary release tar\"\n  download-or-bust \"${server_binary_tar_hash}\" \"${server_binary_tar_urls[@]}\"\n  tar xzf \"${KUBE_HOME}/${server_binary_tar}\" -C \"${KUBE_HOME}\" --overwrite\n  # Copy docker_tag and image files to ${KUBE_HOME}/kube-docker-files.\n  src_dir=\"${KUBE_HOME}/kubernetes/server/bin\"\n  dst_dir=\"${KUBE_HOME}/kube-docker-files\"\n  mkdir -p \"${dst_dir}\"\n  cp \"${src_dir}/\"*.docker_tag \"${dst_dir}\"\n  if [[ \"${KUBERNETES_MASTER:-}\" == \"false\" ]]; then\n    cp \"${src_dir}/kube-proxy.tar\" \"${dst_dir}\"\n    if [[ \"${ENABLE_NODE_PROBLEM_DETECTOR:-}\" == \"standalone\" ]]; then\n      install-node-problem-detector\n    fi\n  else\n    cp \"${src_dir}/kube-apiserver.tar\" \"${dst_dir}\"\n    cp \"${src_dir}/kube-controller-manager.tar\" \"${dst_dir}\"\n    cp \"${src_dir}/kube-scheduler.tar\" \"${dst_dir}\"\n    cp -r \"${KUBE_HOME}/kubernetes/addons\" \"${dst_dir}\"\n  fi\n  local -r kube_bin=\"${KUBE_HOME}/bin\"\n  mv \"${src_dir}/kubelet\" \"${kube_bin}\"\n  mv \"${src_dir}/kubectl\" \"${kube_bin}\"\n\n  if [[ \"${NETWORK_PROVIDER:-}\" == \"kubenet\" ]] || \\\n     [[ \"${NETWORK_PROVIDER:-}\" == \"cni\" ]]; then\n    #TODO(andyzheng0831): We should make the cni version number as a k8s env variable.\n    local -r cni_tar=\"cni-0799f5732f2a11b329d9e3d51b9c8f2e3759f2ff.tar.gz\"\n    local -r cni_sha1=\"1d9788b0f5420e1a219aad2cb8681823fc515e7c\"\n    download-or-bust \"${cni_sha1}\" \"https://storage.googleapis.com/kubernetes-release/network-plugins/${cni_tar}\"\n    local -r cni_dir=\"${KUBE_HOME}/cni\"\n    mkdir -p \"${cni_dir}\"\n    tar xzf \"${KUBE_HOME}/${cni_tar}\" -C \"${cni_dir}\" --overwrite\n    mv \"${cni_dir}/bin\"/* \"${kube_bin}\"\n    rmdir \"${cni_dir}/bin\"\n    rm -f \"${KUBE_HOME}/${cni_tar}\"\n  fi\n\n  mv \"${KUBE_HOME}/kubernetes/LICENSES\" \"${KUBE_HOME}\"\n  mv \"${KUBE_HOME}/kubernetes/kubernetes-src.tar.gz\" \"${KUBE_HOME}\"\n\n  # Put kube-system pods manifests in ${KUBE_HOME}/kube-manifests/.\n  dst_dir=\"${KUBE_HOME}/kube-manifests\"\n  mkdir -p \"${dst_dir}\"\n  local -r manifests_tar_urls=( $(split-commas \"${KUBE_MANIFESTS_TAR_URL}\") )\n  local -r manifests_tar=\"${manifests_tar_urls[0]##*/}\"\n  if [ -n \"${KUBE_MANIFESTS_TAR_HASH:-}\" ]; then\n    local -r manifests_tar_hash=\"${KUBE_MANIFESTS_TAR_HASH}\"\n  else\n    echo \"Downloading k8s manifests sha1 (not found in env)\"\n    download-or-bust \"\" \"${manifests_tar_urls[@]/.tar.gz/.tar.gz.sha1}\"\n    local -r manifests_tar_hash=$(cat \"${manifests_tar}.sha1\")\n  fi\n  echo \"Downloading k8s manifests tar\"\n  download-or-bust \"${manifests_tar_hash}\" \"${manifests_tar_urls[@]}\"\n  tar xzf \"${KUBE_HOME}/${manifests_tar}\" -C \"${dst_dir}\" --overwrite\n  local -r kube_addon_registry=\"${KUBE_ADDON_REGISTRY:-gcr.io/google_containers}\"\n  if [[ \"${kube_addon_registry}\" != \"gcr.io/google_containers\" ]]; then\n    find \"${dst_dir}\" -name \\*.yaml -or -name \\*.yaml.in | \\\n      xargs sed -ri \"s@(image:\\s.*)gcr.io/google_containers@\\1${kube_addon_registry}@\"\n    find \"${dst_dir}\" -name \\*.manifest -or -name \\*.json | \\\n      xargs sed -ri \"s@(image\\\":\\s+\\\")gcr.io/google_containers@\\1${kube_addon_registry}@\"\n  fi\n  cp \"${dst_dir}/kubernetes/gci-trusty/gci-configure-helper.sh\" \"${KUBE_HOME}/bin/configure-helper.sh\"\n  cp \"${dst_dir}/kubernetes/gci-trusty/health-monitor.sh\" \"${KUBE_HOME}/bin/health-monitor.sh\"\n  chmod -R 755 \"${kube_bin}\"\n\n  # Install gci mounter related artifacts to allow mounting storage volumes in GCI\n  install-gci-mounter-tools\n  \n  # Clean up.\n  rm -rf \"${KUBE_HOME}/kubernetes\"\n  rm -f \"${KUBE_HOME}/${server_binary_tar}\"\n  rm -f \"${KUBE_HOME}/${server_binary_tar}.sha1\"\n  rm -f \"${KUBE_HOME}/${manifests_tar}\"\n  rm -f \"${KUBE_HOME}/${manifests_tar}.sha1\"\n}\n\n######### Main Function ##########\necho \"Start to install kubernetes files\"\nset-broken-motd\nKUBE_HOME=\"/home/kubernetes\"\ndownload-kube-env\nsource \"${KUBE_HOME}/kube-env\"\nif [[ \"${KUBERNETES_MASTER:-}\" == \"true\" ]]; then\n  download-kube-master-certs\nfi\ninstall-kube-binary-config\necho \"Done for installing kubernetes files\"\n\n"
    },
    {
     "key": "cluster-name",
     "value": "cluster-1"
    }
   ]
  },
  "serviceAccounts": [
   {
    "email": "default",
    "scopes": [
     "https://www.googleapis.com/auth/compute",
     "https://www.googleapis.com/auth/devstorage.read_only",
     "https://www.googleapis.com/auth/logging.write",
     "https://www.googleapis.com/auth/monitoring.write",
     "https://www.googleapis.com/auth/servicecontrol",
     "https://www.googleapis.com/auth/service.management.readonly",
     "https://www.googleapis.com/auth/trace.append"
    ]
   }
  ],
  "scheduling": {
   "onHostMaintenance": "MIGRATE",
   "automaticRestart": true,
   "preemptible": false
  }
 },
 "selfLink": "https://www.googleapis.com/compute/v1/projects/krzysztof-jastrzebski-dev/global/instanceTemplates/gke-cluster-1-default-pool-f7607aac"
}`

const machineType = `{
 "kind": "compute#machineType",
 "id": "3001",
 "creationTimestamp": "2015-01-16T09:25:43.314-08:00",
 "name": "n1-standard-1",
 "description": "1 vCPU, 3.75 GB RAM",
 "guestCpus": 1,
 "memoryMb": 3840,
 "maximumPersistentDisks": 32,
 "maximumPersistentDisksSizeGb": "65536",
 "zone": "us-central1-b",
 "selfLink": "https://www.googleapis.com/compute/v1/projects/krzysztof-jastrzebski-dev/zones/us-central1-b/machineTypes/n1-standard-1",
 "isSharedCpu": false
}
`

// CreateGceManager constructs gceManager object.
func newTestGceManager(t *testing.T, testServerURL string, mode GcpCloudProviderMode) *GceManager {
	client := &http.Client{}
	gceService, err := gce.New(client)
	assert.NoError(t, err)
	gceService.BasePath = testServerURL
	manager := &GceManager{
		migs:        make([]*migInformation, 0),
		gceService:  gceService,
		migCache:    make(map[GceRef]*Mig),
		zone:        zone,
		projectId:   projectId,
		clusterName: clusterName,
		mode:        mode,
		templates: &templateBuilder{
			projectId: projectId,
			zone:      zone,
			service:   gceService,
		},
	}

	if mode == ModeGKE {
		gkeService, err := gke.New(client)
		assert.NoError(t, err)
		gkeService.BasePath = testServerURL
		manager.gkeService = gkeService

	}
	return manager
}

func TestFetchAllNodePools(t *testing.T) {
	server := NewHttpServerMock()
	g := newTestGceManager(t, server.URL, ModeGKE)

	// Fetch one node pool.
	server.On("handle", "/v1/projects/project1/zones/us-central1-b/clusters/cluster1/nodePools").Return(allNodePools1).Once()
	server.On("handle", "/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool").Return(instanceGroupManager).Once()
	server.On("handle", "/project1/global/instanceTemplates/gke-cluster-1-default-pool").Return(instanceTemplate).Once()
	server.On("handle", "/project1/zones/us-central1-b/machineTypes/n1-standard-1").Return(machineType).Once()

	err := g.fetchAllNodePools()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(g.migs))

	// Fetch three node pools, skip one.
	server.On("handle", "/v1/projects/project1/zones/us-central1-b/clusters/cluster1/nodePools").Return(allNodePools2).Once()
	server.On("handle", "/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool").Return(instanceGroupManager).Once()
	server.On("handle", "/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-node_pool2").Return(instanceGroupManager).Once()
	server.On("handle", "/project1/global/instanceTemplates/gke-cluster-1-default-pool").Return(instanceTemplate).Once()
	server.On("handle", "/project1/global/instanceTemplates/gke-cluster-1-default-pool").Return(instanceTemplate).Once()
	server.On("handle", "/project1/zones/us-central1-b/machineTypes/n1-standard-1").Return(machineType).Once()
	server.On("handle", "/project1/zones/us-central1-b/machineTypes/n1-standard-1").Return(machineType).Once()

	err = g.fetchAllNodePools()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(g.migs))

	// Fetch one node pool, remove node pool registered in previous step.
	server.On("handle", "/v1/projects/project1/zones/us-central1-b/clusters/cluster1/nodePools").Return(allNodePools1).Once()
	server.On("handle", "/project1/zones/us-central1-a/instanceGroupManagers/gke-cluster-1-default-pool").Return(instanceGroupManager).Once()
	server.On("handle", "/project1/global/instanceTemplates/gke-cluster-1-default-pool").Return(instanceTemplate).Once()
	server.On("handle", "/project1/zones/us-central1-b/machineTypes/n1-standard-1").Return(machineType).Once()
	err = g.fetchAllNodePools()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(g.migs))
}
