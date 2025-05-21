package dependencygraph

import (
	"encoding/json"
	"fmt"
	"github.com/kendru/darwin/go/depgraph"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type GenericNode struct {
	Parent *string `json:"depends_on"`
}

func (g *GenericNode) GetParent() *string {
	return g.Parent
}

type NodeList map[string]GenericNode

func TestDependencyGraph(t *testing.T) {

	data := `
{
  "alectoropodous-retrogradely": {
    "image": "quay.io/krkn-chaos/krkn-hub:pod-network-chaos",
    "name": "pod-network-chaos",
    "env": {
      "EGRESS_PORTS": "[]",
      "INGRESS_PORTS": "[]",
      "INSTANCE_COUNT": "1",
      "LABEL_SELECTOR": "<When label_selector is not specified, pod matching the name will be selected for the chaos scenario>",
      "NAMESPACE": "<Namespace of the pod to which filter need to be applied(required)>",
      "TEST_DURATION": "120",
      "TRAFFIC_TYPE": "[ingress,egress]",
      "WAIT_DURATION": "300"
    },
    "volumes": {}
  },
  "bathysophical-hunh": {
    "image": "quay.io/krkn-chaos/krkn-hub:pod-scenarios",
    "name": "pod-scenarios",
    "env": {
      "DISRUPTION_COUNT": "1",
      "EXPECTED_RECOVERY_TIME": "120",
      "KILL_TIMEOUT": "180",
      "NAMESPACE": "openshift-*",
      "NAME_PATTERN": ".*",
      "POD_LABEL": "<Label of the pod(s) to target>"
    },
    "volumes": {}
  },
  "durant-trafficless": {
    "image": "quay.io/krkn-chaos/krkn-hub:time-scenarios",
    "name": "time-scenarios",
    "env": {
      "ACTION": "skew_date",
      "CONTAINER_NAME": "<Container in the specified pod to target in case the pod has multiple containers running. Random container is picked if empty>",
      "LABEL_SELECTOR": "k8s-app=etcd",
      "NAMESPACE": "<Namespace of the pods you want to skew, need to be set only if setting a specific pod name>",
      "OBJECT_NAME": "[]",
      "OBJECT_TYPE": "pod"
    },
    "volumes": {}
  },
  "impassionment-unordinateness": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-scenarios",
    "name": "node-scenarios",
    "env": {
      "ACTION": "<action performed on the node, visit https://github.com/krkn-chaos/krkn/blob/main/docs/node_scenarios.md for more infos(required)>",
      "AWS_ACCESS_KEY_ID": "<AWS Access Key ID>",
      "AWS_DEFAULT_REGION": "<AWS default region>",
      "AWS_SECRET_ACCESS_KEY": "<AWS Secret Access Key>",
      "AZURE_CLIENT_ID": "<IBM Cloud API Key>",
      "AZURE_TENANT_ID": "<Azure Tenant>",
      "BMC_ADDR": "<Only needed for Baremetal ( bm ) - IPMI/bmc address>",
      "BMC_PASSWORD": "<Only needed for Baremetal ( bm ) - IPMI/bmc password>",
      "BMC_USER": "<Only needed for Baremetal ( bm ) - IPMI/bmc username>",
      "CLOUD_TYPE": "aws",
      "DURATION": "120",
      "IBMC_APIKEY": "<IBM Cloud API Key>",
      "IBMC_URL": "<IBM Cloud URL>",
      "INSTANCE_COUNT": "1",
      "LABEL_SELECTOR": "node-role.kubernetes.io/worker",
      "NODE_NAME": "<Node name to inject faults in case of targeting a specific node; Can set multiple node names separated by a comma>",
      "RUNS": "1",
      "SKIP_OPENSHIFT_CHECKS": "False",
      "TIMEOUT": "180",
      "VERIFY_SESSION": "False",
      "VSPHERE_IP": "<VSpere IP Address>",
      "VSPHERE_PASSWORD": "<VSpere password>",
      "VSPHERE_USERNAME": "<VSpere IP Address>"
    },
    "volumes": {}
  },
  "individualist-creatine": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-io-hog",
    "name": "node-io-hog",
    "env": {
      "IO_BLOCK_SIZE": "1m",
      "IO_WORKERS": "5",
      "IO_WRITE_BYTES": "10m",
      "NAMESPACE": "default",
      "NODE_SELECTORS": "<Node selectors where the scenario containers will be scheduled in the format \"<selector>=<value>\". NOTE: This value can be specified as a list of node selectors separated by \";\". Will be instantiated a container per each node selector with the same scenario options. This option is meant to run one or more stress scenarios simultaneously on different nodes, kubernetes will schedule the pods on the target node accordingly with the selector specified. Specifying the same selector multiple times will instantiate as many scenario container as the number of times the selector is specified on the same node>",
      "TOTAL_CHAOS_DURATION": "60"
    },
    "volumes": {}
  },
  "indivision-predetermination": {
    "image": "quay.io/krkn-chaos/krkn-hub:network-chaos",
    "name": "network-chaos",
    "env": {
      "DURATION": "300",
      "EGRESS": "{bandwidth: 100mbit}",
      "EXECUTION": "parallel",
      "INSTANCE_COUNT": "1",
      "INTERFACES": "[]",
      "LABEL_SELECTOR": "node-role.kubernetes.io/master",
      "NETWORK_PARAMS": "<latency, loss and bandwidth are the three supported network parameters to alter for the chaos test. For example: {latency: 50ms, loss: 0.02}>",
      "NODE_NAME": "<Node name to inject faults in case of targeting a specific node; Can set multiple node names separated by a comma>",
      "TARGET_NODE_AND_INTERFACE": "<Dictionary with key as node name(s) and value as a list of its interfaces to test. For example: {ip-10-0-216-2.us-west-2.compute.internal: [ens5]}>",
      "TRAFFIC_TYPE": "<Selects the network chaos scenario type can be ingress or egress(required)>",
      "WAIT_DURATION": "300"
    },
    "volumes": {}
  },
  "mealymouthedly-coelogastrula": {
    "image": "quay.io/krkn-chaos/krkn-hub:container-scenarios",
    "name": "container-scenarios",
    "env": {
      "ACTION": "1",
      "CONTAINER_NAME": "etcd",
      "DISRUPTION_COUNT": "1",
      "EXPECTED_RECOVERY_TIME": "60",
      "LABEL_SELECTOR": "k8s-app=etcd",
      "NAMESPACE": "openshift-etcd"
    },
    "volumes": {}
  },
  "metaphragmal-necropolis": {
    "image": "quay.io/krkn-chaos/krkn-hub:power-outages",
    "name": "power-outages",
    "env": {
      "AWS_DEFAULT_REGION": "<AWS default region>",
      "AWS_SECRET_ACCESS_KEY": "<AWS Secret Access Key>",
      "CLOUD_TYPE": "aws",
      "SHUTDOWN_DURATION": "1200",
      "TIMEOUT": "600",
      "VSPHERE_PASSWORD": "<AWS Secret Access Key>"
    },
    "volumes": {}
  },
  "novelistic-straticulation": {
    "image": "quay.io/krkn-chaos/krkn-hub:pvc-scenarios",
    "name": "pvc-scenarios",
    "env": {
      "DURATION": "60",
      "FILL_PERCENTAGE": "50",
      "NAMESPACE": "<Targeted namespace in the cluster (required)(required)>",
      "POD_NAME": "<Targeted pod in the cluster (if null, PVC_NAME is required)>",
      "PVC_NAME": "<Targeted PersistentVolumeClaim in the cluster (if null, POD_NAME is required)>"
    },
    "volumes": {}
  },
  "novemberish-loa": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-cpu-hog",
    "name": "node-cpu-hog",
    "env": {
      "NAMESPACE": "default",
      "NODE_CPU_CORE": "2",
      "NODE_CPU_PERCENTAGE": "50",
      "NODE_SELECTORS": "<Node selectors where the scenario containers will be scheduled in the format \"<selector>=<value>\". NOTE: This value can be specified as a list of node selectors separated by \";\". Will be instantiated a container per each node selector with the same scenario options. This option is meant to run one or more stress scenarios simultaneously on different nodes, kubernetes will schedule the pods on the target node accordingly with the selector specified. Specifying the same selector multiple times will instantiate as many scenario container as the number of times the selector is specified on the same node>",
      "TOTAL_CHAOS_DURATION": "60"
    },
    "volumes": {}
  },
  "pharmacoendocrinology-slurp": {
    "image": "quay.io/krkn-chaos/krkn-hub:application-outages",
    "name": "application-outages",
    "env": {
      "BLOCK_TRAFFIC_TYPE": "[Ingress, Egress]",
      "DURATION": "600",
      "NAMESPACE": "<Namespace to target - all application routes will go inaccessible if pod selector is empty ( Required )(required)>",
      "POD_SELECTOR": "<Pods to target. For example \"{app: foo}\"(required)>"
    },
    "volumes": {}
  },
  "precedential-desiccative": {
    "image": "quay.io/krkn-chaos/krkn-hub:zone-outages",
    "name": "zone-outages",
    "env": {
      "AWS_ACCESS_KEY_ID": "<AWS Access Key ID>",
      "AWS_DEFAULT_REGION": "<AWS default region>",
      "AWS_SECRET_ACCESS_KEY": "<AWS Secret Access Key>",
      "CLOUD_TYPE": "aws",
      "DURATION": "600",
      "SUBNET_ID": "<subnet-id to deny both ingress and egress traffic ( REQUIRED ). Format: [subnet1, subnet2]>",
      "VPC_ID": "<cluster virtual private network to target(required)>"
    },
    "volumes": {}
  },
  "shirallee-marikina": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-memory-hog",
    "name": "node-memory-hog",
    "env": {
      "MEMORY_CONSUMPTION_PERCENTAGE": "90%",
      "NAMESPACE": "default",
      "NODE_SELECTORS": "<Node selectors where the scenario containers will be scheduled in the format \"<selector>=<value>\". NOTE: This value can be specified as a list of node selectors separated by \";\". Will be instantiated a container per each node selector with the same scenario options. This option is meant to run one or more stress scenarios simultaneously on different nodes, kubernetes will schedule the pods on the target node accordingly with the selector specified. Specifying the same selector multiple times will instantiate as many scenario container as the number of times the selector is specified on the same node>",
      "NUMBER_OF_WORKERS": "1",
      "TOTAL_CHAOS_DURATION": "60"
    },
    "volumes": {}
  },
  "thinglike-temse": {
    "image": "quay.io/krkn-chaos/krkn-hub:service-disruption-scenarios",
    "name": "service-disruption-scenarios",
    "env": {
      "DELETE_COUNT": "1",
      "LABEL_SELECTOR": "<Label of the namespace to target. Set this parameter only if NAMESPACE is not set>",
      "NAMESPACE": "openshift-etcd",
      "RUNS": "1"
    },
    "volumes": {}
  },
  "unrefracting-hierophantic": {
    "image": "quay.io/krkn-chaos/krkn-hub:service-hijacking",
    "name": "service-hijacking",
    "env": {
      "SCENARIO_BASE64": "<The absolute path of the scenario file compiled following the documentation(required)>"
    },
    "volumes": {}
  }
}
`
	testStruct := make(NodeList)
	graph := depgraph.New()
	err := json.Unmarshal([]byte(data), &testStruct)
	assert.Nil(t, err)

	for k, v := range testStruct {
		if parent := v.GetParent(); parent != nil {
			err := graph.DependOn(k, *parent)
			assert.Nil(t, err)
		}

	}

	for i, layer := range graph.TopoSortedLayers() {
		fmt.Printf("%d: %s\n", i, strings.Join(layer, ", "))
	}

}
