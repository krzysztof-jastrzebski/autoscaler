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

package core

import (
	"k8s.io/autoscaler/cluster-autoscaler/config/dynamic"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	kube_util "k8s.io/autoscaler/cluster-autoscaler/utils/kubernetes"
	"k8s.io/autoscaler/cluster-autoscaler/utils/pods"
	kube_client "k8s.io/client-go/kubernetes"
	kube_record "k8s.io/client-go/tools/record"
)

// AutoscalerBuilderOptions contain various options to customize how autoscaler is created.
type AutoscalerBuilderOptions struct {
	AutoscalingOptions AutoscalingOptions
	KubeClient         kube_client.Interface
	KubeEventRecorder  kube_record.EventRecorder
	PredicateChecker   *simulator.PredicateChecker
	ListerRegistry     kube_util.ListerRegistry
	PodListProcessor   pods.PodListProcessor
}

// AutoscalerBuilder builds an instance of Autoscaler which is the core of CA
type AutoscalerBuilder interface {
	SetDynamicConfig(config dynamic.Config) AutoscalerBuilder
	Build() (Autoscaler, errors.AutoscalerError)
}

// AutoscalerBuilderImpl builds new autoscalers from its state including initial `AutoscalingOptions` given at startup and
// `dynamic.Config` read on demand from the configmap
type AutoscalerBuilderImpl struct {
	options       AutoscalerBuilderOptions
	dynamicConfig *dynamic.Config
}

// NewAutoscalerBuilder builds an AutoscalerBuilder from required parameters
func NewAutoscalerBuilder(options AutoscalerBuilderOptions) *AutoscalerBuilderImpl {
	builder := &AutoscalerBuilderImpl{
		options: options,
	}
	if builder.options.PodListProcessor == nil {
		builder.options.PodListProcessor = pods.NewPodListProcessor()
	}
	return builder
}

// SetDynamicConfig sets an instance of dynamic.Config read from a configmap so that
// the new autoscaler built afterwards reflect the latest configuration contained in the configmap
func (b *AutoscalerBuilderImpl) SetDynamicConfig(config dynamic.Config) AutoscalerBuilder {
	b.dynamicConfig = &config
	return b
}

// Build an autoscaler according to the builder's state
func (b *AutoscalerBuilderImpl) Build() (Autoscaler, errors.AutoscalerError) {
	if b.dynamicConfig != nil {
		c := *(b.dynamicConfig)
		b.options.AutoscalingOptions.NodeGroups = c.NodeGroupSpecStrings()
	}
	return NewStaticAutoscaler(
		b.options.AutoscalingOptions,
		b.options.PredicateChecker,
		b.options.KubeClient,
		b.options.KubeEventRecorder,
		b.options.ListerRegistry,
		b.options.PodListProcessor)
}
