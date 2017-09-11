/*
Copyright 2016 The Kubernetes Authors.

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

package core

import (
	"fmt"
	"sort"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testprovider "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/test"
	"k8s.io/autoscaler/cluster-autoscaler/clusterstate"
	"k8s.io/autoscaler/cluster-autoscaler/clusterstate/utils"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	kube_util "k8s.io/autoscaler/cluster-autoscaler/utils/kubernetes"
	. "k8s.io/autoscaler/cluster-autoscaler/utils/test"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

func TestFindUnneededNodes(t *testing.T) {
	p1 := BuildTestPod("p1", 100, 0)
	p1.Spec.NodeName = "n1"

	// shared owner reference
	ownerRef := GenerateOwnerReferences("rs", "ReplicaSet", "extensions/v1beta1", "")

	p2 := BuildTestPod("p2", 300, 0)
	p2.Spec.NodeName = "n2"
	p2.OwnerReferences = ownerRef

	p3 := BuildTestPod("p3", 400, 0)
	p3.OwnerReferences = ownerRef
	p3.Spec.NodeName = "n3"

	p4 := BuildTestPod("p4", 2000, 0)
	p4.OwnerReferences = ownerRef
	p4.Spec.NodeName = "n4"

	p5 := BuildTestPod("p5", 100, 0)
	p5.OwnerReferences = ownerRef
	p5.Spec.NodeName = "n5"

	n1 := BuildTestNode("n1", 1000, 10)
	n2 := BuildTestNode("n2", 1000, 10)
	n3 := BuildTestNode("n3", 1000, 10)
	n4 := BuildTestNode("n4", 10000, 10)
	n5 := BuildTestNode("n5", 1000, 10)
	n5.Annotations = map[string]string{
		ScaleDownDisabledKey: "true",
	}

	SetNodeReadyState(n1, true, time.Time{})
	SetNodeReadyState(n2, true, time.Time{})
	SetNodeReadyState(n3, true, time.Time{})
	SetNodeReadyState(n4, true, time.Time{})
	SetNodeReadyState(n5, true, time.Time{})

	fakeClient := &fake.Clientset{}
	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)
	provider.AddNode("ng1", n3)
	provider.AddNode("ng1", n4)
	provider.AddNode("ng1", n5)

	context := AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.35,
		},
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		LogRecorder:          fakeLogRecorder,
		CloudProvider:        provider,
	}
	sd := NewScaleDown(&context)
	sd.UpdateUnneededNodes([]*apiv1.Node{n1, n2, n3, n4, n5}, []*apiv1.Node{n1, n2, n3, n4, n5}, []*apiv1.Pod{p1, p2, p3, p4}, time.Now(), nil)

	assert.Equal(t, 1, len(sd.unneededNodes))
	addTime, found := sd.unneededNodes["n2"]
	assert.True(t, found)
	assert.Contains(t, sd.podLocationHints, p2.Namespace+"/"+p2.Name)
	assert.Equal(t, 4, len(sd.nodeUtilizationMap))

	sd.unremovableNodes = make(map[string]time.Time)
	sd.unneededNodes["n1"] = time.Now()
	sd.UpdateUnneededNodes([]*apiv1.Node{n1, n2, n3, n4}, []*apiv1.Node{n1, n2, n3, n4}, []*apiv1.Pod{p1, p2, p3, p4}, time.Now(), nil)
	sd.unremovableNodes = make(map[string]time.Time)

	assert.Equal(t, 1, len(sd.unneededNodes))
	addTime2, found := sd.unneededNodes["n2"]
	assert.True(t, found)
	assert.Equal(t, addTime, addTime2)
	assert.Equal(t, 4, len(sd.nodeUtilizationMap))

	sd.unremovableNodes = make(map[string]time.Time)
	sd.UpdateUnneededNodes([]*apiv1.Node{n1, n2, n3, n4}, []*apiv1.Node{n1, n3, n4}, []*apiv1.Pod{p1, p2, p3, p4}, time.Now(), nil)
	assert.Equal(t, 0, len(sd.unneededNodes))

	// Node n1 is unneeded, but should be skipped because it has just recently been found to be unremovable
	sd.UpdateUnneededNodes([]*apiv1.Node{n1}, []*apiv1.Node{n1}, []*apiv1.Pod{}, time.Now(), nil)
	assert.Equal(t, 0, len(sd.unneededNodes))

	// But it should be checked after timeout
	sd.UpdateUnneededNodes([]*apiv1.Node{n1}, []*apiv1.Node{n1}, []*apiv1.Pod{}, time.Now().Add(UnremovableNodeRecheckTimeout+time.Second), nil)
	assert.Equal(t, 1, len(sd.unneededNodes))
}

func TestFindUnneededMaxCandidates(t *testing.T) {
	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 100, 2)

	numNodes := 100
	nodes := make([]*apiv1.Node, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		n := BuildTestNode(fmt.Sprintf("n%v", i), 1000, 10)
		SetNodeReadyState(n, true, time.Time{})
		provider.AddNode("ng1", n)
		nodes = append(nodes, n)
	}

	// shared owner reference
	ownerRef := GenerateOwnerReferences("rs", "ReplicaSet", "extensions/v1beta1", "")

	pods := make([]*apiv1.Pod, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		p := BuildTestPod(fmt.Sprintf("p%v", i), 100, 0)
		p.Spec.NodeName = fmt.Sprintf("n%v", i)
		p.OwnerReferences = ownerRef
		pods = append(pods, p)
	}

	fakeClient := &fake.Clientset{}
	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)

	numCandidates := 30

	context := AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold:    0.35,
			ScaleDownNonEmptyCandidatesCount: numCandidates,
			ScaleDownCandidatesPoolRatio:     1,
			ScaleDownCandidatesPoolMinCount:  1000,
		},
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		LogRecorder:          fakeLogRecorder,
		CloudProvider:        provider,
	}
	sd := NewScaleDown(&context)

	sd.UpdateUnneededNodes(nodes, nodes, pods, time.Now(), nil)
	assert.Equal(t, numCandidates, len(sd.unneededNodes))
	// Simulate one of the unneeded nodes got deleted
	deleted := sd.unneededNodesList[len(sd.unneededNodesList)-1]
	for i, node := range nodes {
		if node.Name == deleted.Name {
			// Move pod away from the node
			var newNode int
			if i >= 1 {
				newNode = i - 1
			} else {
				newNode = i + 1
			}
			pods[i].Spec.NodeName = nodes[newNode].Name
			nodes[i] = nodes[len(nodes)-1]
			nodes[len(nodes)-1] = nil
			nodes = nodes[:len(nodes)-1]
			break
		}
	}

	sd.UpdateUnneededNodes(nodes, nodes, pods, time.Now(), nil)
	// Check that the deleted node was replaced
	assert.Equal(t, numCandidates, len(sd.unneededNodes))
	assert.NotContains(t, sd.unneededNodes, deleted)
}

func TestFindUnneededEmptyNodes(t *testing.T) {
	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 100, 100)

	// 30 empty nodes and 70 heavily underutilized.
	numNodes := 100
	numEmpty := 30
	nodes := make([]*apiv1.Node, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		n := BuildTestNode(fmt.Sprintf("n%v", i), 1000, 10)
		SetNodeReadyState(n, true, time.Time{})
		provider.AddNode("ng1", n)
		nodes = append(nodes, n)
	}

	// shared owner reference
	ownerRef := GenerateOwnerReferences("rs", "ReplicaSet", "extensions/v1beta1", "")

	pods := make([]*apiv1.Pod, 0, numNodes)
	for i := 0; i < numNodes-numEmpty; i++ {
		p := BuildTestPod(fmt.Sprintf("p%v", i), 100, 0)
		p.Spec.NodeName = fmt.Sprintf("n%v", i)
		p.OwnerReferences = ownerRef
		pods = append(pods, p)
	}

	fakeClient := &fake.Clientset{}
	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)

	numCandidates := 30

	context := AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold:    0.35,
			ScaleDownNonEmptyCandidatesCount: numCandidates,
			ScaleDownCandidatesPoolRatio:     1.0,
			ScaleDownCandidatesPoolMinCount:  1000,
		},
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		LogRecorder:          fakeLogRecorder,
		CloudProvider:        provider,
	}
	sd := NewScaleDown(&context)

	sd.UpdateUnneededNodes(nodes, nodes, pods, time.Now(), nil)
	for _, node := range sd.unneededNodesList {
		t.Log(node.Name)
	}
	assert.Equal(t, numEmpty+numCandidates, len(sd.unneededNodes))
}

func TestFindUnneededNodePool(t *testing.T) {
	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 100, 100)

	numNodes := 100
	nodes := make([]*apiv1.Node, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		n := BuildTestNode(fmt.Sprintf("n%v", i), 1000, 10)
		SetNodeReadyState(n, true, time.Time{})
		provider.AddNode("ng1", n)
		nodes = append(nodes, n)
	}

	// shared owner reference
	ownerRef := GenerateOwnerReferences("rs", "ReplicaSet", "extensions/v1beta1", "")

	pods := make([]*apiv1.Pod, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		p := BuildTestPod(fmt.Sprintf("p%v", i), 100, 0)
		p.Spec.NodeName = fmt.Sprintf("n%v", i)
		p.OwnerReferences = ownerRef
		pods = append(pods, p)
	}

	fakeClient := &fake.Clientset{}
	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)

	numCandidates := 30

	context := AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold:    0.35,
			ScaleDownNonEmptyCandidatesCount: numCandidates,
			ScaleDownCandidatesPoolRatio:     0.1,
			ScaleDownCandidatesPoolMinCount:  10,
		},
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		LogRecorder:          fakeLogRecorder,
		CloudProvider:        provider,
	}
	sd := NewScaleDown(&context)

	sd.UpdateUnneededNodes(nodes, nodes, pods, time.Now(), nil)
	assert.NotEmpty(t, sd.unneededNodes)
}

func TestDrainNode(t *testing.T) {
	deletedPods := make(chan string, 10)
	updatedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}

	p1 := BuildTestPod("p1", 100, 0)
	p2 := BuildTestPod("p2", 300, 0)
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})

	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		return true, n1, nil
	})
	fakeClient.Fake.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		createAction := action.(core.CreateAction)
		if createAction == nil {
			return false, nil, nil
		}
		eviction := createAction.GetObject().(*policyv1.Eviction)
		if eviction == nil {
			return false, nil, nil
		}
		deletedPods <- eviction.Name
		return true, nil, nil
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})
	err := drainNode(n1, []*apiv1.Pod{p1, p2}, fakeClient, kube_util.CreateEventRecorder(fakeClient), 20, 5*time.Second, 0*time.Second)
	assert.NoError(t, err)
	deleted := make([]string, 0)
	deleted = append(deleted, getStringFromChan(deletedPods))
	deleted = append(deleted, getStringFromChan(deletedPods))
	sort.Strings(deleted)
	assert.Equal(t, p1.Name, deleted[0])
	assert.Equal(t, p2.Name, deleted[1])
	assert.Equal(t, n1.Name, getStringFromChan(updatedNodes))
}

func TestDrainNodeWithRetries(t *testing.T) {
	deletedPods := make(chan string, 10)
	updatedNodes := make(chan string, 10)
	// Simulate pdb of size 1, by making them goroutine succeed sequentially
	// and fail/retry before they can proceed.
	ticket := make(chan bool, 1)
	fakeClient := &fake.Clientset{}

	p1 := BuildTestPod("p1", 100, 0)
	p2 := BuildTestPod("p2", 300, 0)
	p3 := BuildTestPod("p3", 300, 0)
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})

	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		return true, n1, nil
	})
	fakeClient.Fake.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		createAction := action.(core.CreateAction)
		if createAction == nil {
			return false, nil, nil
		}
		eviction := createAction.GetObject().(*policyv1.Eviction)
		if eviction == nil {
			return false, nil, nil
		}
		select {
		case <-ticket:
			deletedPods <- eviction.Name
			return true, nil, nil
		default:
			select {
			case ticket <- true:
			default:
			}
			return true, nil, fmt.Errorf("Too many concurrent evictions")
		}
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})
	err := drainNode(n1, []*apiv1.Pod{p1, p2, p3}, fakeClient, kube_util.CreateEventRecorder(fakeClient), 20, 5*time.Second, 0*time.Second)
	assert.NoError(t, err)
	deleted := make([]string, 0)
	deleted = append(deleted, getStringFromChan(deletedPods))
	deleted = append(deleted, getStringFromChan(deletedPods))
	deleted = append(deleted, getStringFromChan(deletedPods))
	sort.Strings(deleted)
	assert.Equal(t, p1.Name, deleted[0])
	assert.Equal(t, p2.Name, deleted[1])
	assert.Equal(t, p3.Name, deleted[2])
	assert.Equal(t, n1.Name, getStringFromChan(updatedNodes))
}

func TestScaleDown(t *testing.T) {
	deletedPods := make(chan string, 10)
	updatedNodes := make(chan string, 10)
	deletedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: "default",
			SelfLink:  "/apivs/extensions/v1beta1/namespaces/default/jobs/job",
		},
	}
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	p1 := BuildTestPod("p1", 100, 0)
	p1.OwnerReferences = GenerateOwnerReferences(job.Name, "Job", "extensions/v1beta1", "")

	p2 := BuildTestPod("p2", 800, 0)
	p1.Spec.NodeName = "n1"
	p2.Spec.NodeName = "n2"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{*p1, *p2}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		deleteAction := action.(core.DeleteAction)
		deletedPods <- deleteAction.GetName()
		return true, nil, nil
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})

	provider := testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		deletedNodes <- node
		return nil
	})
	provider.AddNodeGroup("ng1", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)
	assert.NotNil(t, provider)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.5,
			ScaleDownUnneededTime:         time.Minute,
			MaxGracefulTerminationSec:     60,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		LogRecorder:          fakeLogRecorder,
	}
	scaleDown := NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2},
		[]*apiv1.Node{n1, n2}, []*apiv1.Pod{p1, p2}, time.Now().Add(-5*time.Minute), nil)
	result, err := scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{p1, p2}, nil)
	waitForDeleteToFinish(t, scaleDown)
	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNodeDeleteStarted, result)
	assert.Equal(t, n1.Name, getStringFromChan(deletedNodes))
	assert.Equal(t, n1.Name, getStringFromChan(updatedNodes))
}

func waitForDeleteToFinish(t *testing.T, sd *ScaleDown) {
	for start := time.Now(); time.Now().Sub(start) < 20*time.Second; time.Sleep(100 * time.Millisecond) {
		if !sd.nodeDeleteStatus.IsDeleteInProgress() {
			return
		}
	}
	t.Fatalf("Node delete not finished")
}

func assertSubset(t *testing.T, a []string, b []string) {
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Failed to find %s (from %s) in %v", x, a, b)
		}
	}
}

func TestScaleDownEmptyMultipleNodeGroups(t *testing.T) {
	updatedNodes := make(chan string, 10)
	deletedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}

	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})

	provider := testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		deletedNodes <- node
		return nil
	})
	provider.AddNodeGroup("ng1", 0, 10, 2)
	provider.AddNodeGroup("ng2", 0, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng2", n2)
	assert.NotNil(t, provider)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.5,
			ScaleDownUnneededTime:         time.Minute,
			MaxGracefulTerminationSec:     60,
			MaxEmptyBulkDelete:            10,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		LogRecorder:          fakeLogRecorder,
	}
	scaleDown := NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2},
		[]*apiv1.Node{n1, n2}, []*apiv1.Pod{}, time.Now().Add(-5*time.Minute), nil)
	result, err := scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{}, nil)
	waitForDeleteToFinish(t, scaleDown)

	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNodeDeleted, result)
	d1 := getStringFromChan(deletedNodes)
	d2 := getStringFromChan(deletedNodes)
	assertSubset(t, []string{d1, d2}, []string{n1.Name, n2.Name})
}

func TestScaleDownEmptySingleNodeGroup(t *testing.T) {
	updatedNodes := make(chan string, 10)
	deletedNodes := make(chan string, 10)
	fakeClient := &fake.Clientset{}

	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})

	provider := testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		deletedNodes <- node
		return nil
	})
	provider.AddNodeGroup("ng1", 0, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)
	assert.NotNil(t, provider)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.5,
			ScaleDownUnneededTime:         time.Minute,
			MaxGracefulTerminationSec:     60,
			MaxEmptyBulkDelete:            10,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		LogRecorder:          fakeLogRecorder,
	}
	scaleDown := NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2},
		[]*apiv1.Node{n1, n2}, []*apiv1.Pod{}, time.Now().Add(-5*time.Minute), nil)
	result, err := scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{}, nil)
	waitForDeleteToFinish(t, scaleDown)

	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNodeDeleted, result)
	d1 := getStringFromChan(deletedNodes)
	d2 := getStringFromChan(deletedNodes)
	assertSubset(t, []string{d1, d2}, []string{n1.Name, n2.Name})
}

func TestNoScaleDownUnready(t *testing.T) {
	fakeClient := &fake.Clientset{}
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, false, time.Now().Add(-3*time.Minute))
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	p2 := BuildTestPod("p2", 800, 0)
	p2.Spec.NodeName = "n2"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})

	provider := testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		t.Fatalf("Unexpected deletion of %s", node)
		return nil
	})
	provider.AddNodeGroup("ng1", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.5,
			ScaleDownUnneededTime:         time.Minute,
			ScaleDownUnreadyTime:          time.Hour,
			MaxGracefulTerminationSec:     60,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		LogRecorder:          fakeLogRecorder,
	}

	// N1 is unready so it requires a bigger unneeded time.
	scaleDown := NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2},
		[]*apiv1.Node{n1, n2}, []*apiv1.Pod{p2}, time.Now().Add(-5*time.Minute), nil)
	result, err := scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{p2}, nil)
	waitForDeleteToFinish(t, scaleDown)

	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNoUnneeded, result)

	deletedNodes := make(chan string, 10)

	provider = testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		deletedNodes <- node
		return nil
	})
	SetNodeReadyState(n1, false, time.Now().Add(-3*time.Hour))
	provider.AddNodeGroup("ng1", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)

	// N1 has been unready for 2 hours, ok to delete.
	context.CloudProvider = provider
	scaleDown = NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2}, []*apiv1.Node{n1, n2},
		[]*apiv1.Pod{p2}, time.Now().Add(-2*time.Hour), nil)
	result, err = scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{p2}, nil)
	waitForDeleteToFinish(t, scaleDown)

	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNodeDeleteStarted, result)
	assert.Equal(t, n1.Name, getStringFromChan(deletedNodes))
}

func TestScaleDownNoMove(t *testing.T) {
	fakeClient := &fake.Clientset{}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: "default",
			SelfLink:  "/apivs/extensions/v1beta1/namespaces/default/jobs/job",
		},
	}
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, true, time.Time{})

	// N2 is unready so no pods can be moved there.
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, false, time.Time{})

	p1 := BuildTestPod("p1", 100, 0)
	p1.OwnerReferences = GenerateOwnerReferences(job.Name, "Job", "extensions/v1beta1", "")

	p2 := BuildTestPod("p2", 800, 0)
	p1.Spec.NodeName = "n1"
	p2.Spec.NodeName = "n2"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{*p1, *p2}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		t.FailNow()
		return false, nil, nil
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		t.FailNow()
		return false, nil, nil
	})
	provider := testprovider.NewTestCloudProvider(nil, func(nodeGroup string, node string) error {
		t.FailNow()
		return nil
	})
	provider.AddNodeGroup("ng1", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng1", n2)
	assert.NotNil(t, provider)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			ScaleDownUtilizationThreshold: 0.5,
			ScaleDownUnneededTime:         time.Minute,
			ScaleDownUnreadyTime:          time.Hour,
			MaxGracefulTerminationSec:     60,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ClusterStateRegistry: clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder),
		LogRecorder:          fakeLogRecorder,
	}
	scaleDown := NewScaleDown(context)
	scaleDown.UpdateUnneededNodes([]*apiv1.Node{n1, n2}, []*apiv1.Node{n1, n2},
		[]*apiv1.Pod{p1, p2}, time.Now().Add(5*time.Minute), nil)
	result, err := scaleDown.TryToScaleDown([]*apiv1.Node{n1, n2}, []*apiv1.Pod{p1, p2}, nil)
	waitForDeleteToFinish(t, scaleDown)

	assert.NoError(t, err)
	assert.Equal(t, ScaleDownNoUnneeded, result)
}

func getStringFromChan(c chan string) string {
	select {
	case val := <-c:
		return val
	case <-time.After(time.Second * 10):
		return "Nothing returned"
	}
}

func TestCleanUpNodeAutoprovisionedGroups(t *testing.T) {
	n1 := BuildTestNode("n1", 1000, 1000)

	provider := testprovider.NewTestAutoprovisioningCloudProvider(
		nil, nil,
		nil, nil,
		nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNode("ng1", n1)
	assert.NotNil(t, provider)
}