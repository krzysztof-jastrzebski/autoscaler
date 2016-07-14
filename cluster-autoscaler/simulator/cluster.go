/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package simulator

import (
	"flag"
	"fmt"
	"math"

	kube_api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	cmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/golang/glog"
)

var (
	skipNodesWithSystemPods = flag.Bool("skip-nodes-with-system-pods", true,
		"If true cluster autoscaler will never delete nodes with pods from kube-system (except for DeamonSet "+
			"or mirror pods)")
	skipNodesWithLocalStorage = flag.Bool("skip-nodes-with-local-storage", true,
		"If true cluster autoscaler will never delete nodes with pods with local storage, e.g. EmptyDir or HostPath")
)

// FindNodesToRemove finds nodes that can be removed. Returns also an information about good
// rescheduling location for each of the pods.
func FindNodesToRemove(candidates []*kube_api.Node, allNodes []*kube_api.Node, pods []*kube_api.Pod,
	client *kube_client.Client, predicateChecker *PredicateChecker, maxCount int,
	fastCheck bool, oldHints map[string]string) (nodesToRemove []*kube_api.Node, podReschedulingHints map[string]string, finalError error) {

	nodeNameToNodeInfo := schedulercache.CreateNodeNameToInfoMap(pods)
	for _, node := range allNodes {
		if nodeInfo, found := nodeNameToNodeInfo[node.Name]; found {
			nodeInfo.SetNode(node)
		}
	}
	result := make([]*kube_api.Node, 0)

	evaluationType := "Detailed evaluation"
	if fastCheck {
		evaluationType = "Fast evaluation"
	}
	newHints := make(map[string]string, len(oldHints))

candidateloop:
	for _, node := range candidates {
		glog.V(2).Infof("%s: %s for removal", evaluationType, node.Name)

		var podsToRemove []*kube_api.Pod
		var err error

		if fastCheck {
			if nodeInfo, found := nodeNameToNodeInfo[node.Name]; found {
				podsToRemove, err = FastGetPodsToMove(nodeInfo, false, *skipNodesWithSystemPods, *skipNodesWithLocalStorage, kube_api.Codecs.UniversalDecoder())
				if err != nil {
					glog.V(2).Infof("%s: node %s cannot be removed: %v", evaluationType, node.Name, err)
					continue candidateloop
				}
			} else {
				glog.V(2).Infof("%s: nodeInfo for %s not found", evaluationType, node.Name)
				continue candidateloop
			}
		} else {
			drainResult, _, _, err := cmd.GetPodsForDeletionOnNodeDrain(client, node.Name,
				kube_api.Codecs.UniversalDecoder(), false, true)

			if err != nil {
				glog.V(2).Infof("%s: node %s cannot be removed: %v", evaluationType, node.Name, err)
				continue candidateloop
			}
			podsToRemove = make([]*kube_api.Pod, 0, len(drainResult))
			for i := range drainResult {
				podsToRemove = append(podsToRemove, &drainResult[i])
			}
		}
		findProblems := findPlaceFor(node.Name, podsToRemove, allNodes, nodeNameToNodeInfo, predicateChecker, oldHints, newHints)
		if findProblems == nil {
			result = append(result, node)
			glog.V(2).Infof("%s: node %s may be removed", evaluationType, node.Name)
			if len(result) >= maxCount {
				break candidateloop
			}
		} else {
			glog.V(2).Infof("%s: node %s is not suitable for removal %v", evaluationType, node.Name, err)
		}
	}
	return result, newHints, nil
}

// CalculateUtilization calculates utilization of a node, defined as total amount of requested resources divided by capacity.
func CalculateUtilization(node *kube_api.Node, nodeInfo *schedulercache.NodeInfo) (float64, error) {
	cpu, err := calculateUtilizationOfResource(node, nodeInfo, kube_api.ResourceCPU)
	if err != nil {
		return 0, err
	}
	mem, err := calculateUtilizationOfResource(node, nodeInfo, kube_api.ResourceMemory)
	if err != nil {
		return 0, err
	}
	return math.Max(cpu, mem), nil
}

func calculateUtilizationOfResource(node *kube_api.Node, nodeInfo *schedulercache.NodeInfo, resourceName kube_api.ResourceName) (float64, error) {
	nodeCapacity, found := node.Status.Capacity[resourceName]
	if !found {
		return 0, fmt.Errorf("Failed to get %v from %s", resourceName, node.Name)
	}
	if nodeCapacity.MilliValue() == 0 {
		return 0, fmt.Errorf("%v is 0 at %s", resourceName, node.Name)
	}
	podsRequest := resource.MustParse("0")
	for _, pod := range nodeInfo.Pods() {
		for _, container := range pod.Spec.Containers {
			if resourceValue, found := container.Resources.Requests[resourceName]; found {
				podsRequest.Add(resourceValue)
			}
		}
	}
	return float64(podsRequest.MilliValue()) / float64(nodeCapacity.MilliValue()), nil
}

// TODO: We don't need to pass list of nodes here as they are already available in nodeInfos.
func findPlaceFor(bannedNode string, pods []*kube_api.Pod, nodes []*kube_api.Node, nodeInfos map[string]*schedulercache.NodeInfo,
	predicateChecker *PredicateChecker, oldHints map[string]string, newHints map[string]string) error {

	newNodeInfos := make(map[string]*schedulercache.NodeInfo)

	podKey := func(pod *kube_api.Pod) string {
		return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	}

	tryNodeForPod := func(nodename string, pod *kube_api.Pod) bool {
		nodeInfo, found := newNodeInfos[nodename]
		if !found {
			nodeInfo, found = nodeInfos[nodename]
		}
		if found {
			nodeInfo.Node().Status.Allocatable = nodeInfo.Node().Status.Capacity
			err := predicateChecker.CheckPredicates(pod, nodeInfo)
			glog.V(4).Infof("Evaluation %s for %s/%s -> %v", nodename, pod.Namespace, pod.Name, err)
			if err == nil {
				// TODO(mwielgus): Optimize it.
				podsOnNode := nodeInfo.Pods()
				podsOnNode = append(podsOnNode, pod)
				newNodeInfo := schedulercache.NewNodeInfo(podsOnNode...)
				newNodeInfo.SetNode(nodeInfo.Node())
				newNodeInfos[nodename] = newNodeInfo
				newHints[podKey(pod)] = nodename
				return true
			}
		}
		return false
	}

	for _, podptr := range pods {
		newpod := *podptr
		newpod.Spec.NodeName = ""
		pod := &newpod

		foundPlace := false
		glog.V(4).Infof("Looking for place for %s/%s", pod.Namespace, pod.Name)

		hintedNode, hasHint := oldHints[podKey(pod)]
		if hasHint {
			if hintedNode != bannedNode && tryNodeForPod(hintedNode, pod) {
				foundPlace = true
			}
		}
		if !foundPlace {
			// TODO: Sort nodes by utilization
			for _, node := range nodes {
				if node.Name == bannedNode {
					continue
				}
				if tryNodeForPod(node.Name, pod) {
					foundPlace = true
					break
				}
			}
			if !foundPlace {
				return fmt.Errorf("failed to find place for %s", podKey)
			}
		}
	}
	return nil
}