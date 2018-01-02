// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ksr

import (
	"sync"
	"testing"

	"github.com/onsi/gomega"

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	proto "github.com/contiv/vpp/plugins/ksr/model/service"
	"github.com/ligato/cn-infra/flavors/local"
	"time"
)

type ServiceTestVars struct {
	k8sListWatch *mockK8sListWatch
	mockKvWriter *mockKeyProtoValWriter
}

var serviceTestVars = ServiceTestVars{
	k8sListWatch: &mockK8sListWatch{},
	mockKvWriter: newMockKeyProtoValWriter(),
}

func TestServiceReflector(t *testing.T) {
	gomega.RegisterTestingT(t)

	flavorLocal := &local.FlavorLocal{}
	flavorLocal.Inject()

	svcReflector := &ServiceReflector{
		ReflectorDeps: ReflectorDeps{
			Log:          flavorLocal.LoggerFor("service-reflector"),
			K8sClientset: &kubernetes.Clientset{},
			K8sListWatch: serviceTestVars.k8sListWatch,
			Publish:      serviceTestVars.mockKvWriter,
		},
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	err := svcReflector.Init(stopCh, &wg)
	gomega.Expect(err).To(gomega.BeNil())

	t.Run("addDeleteService", testAddDeleteService)
	serviceTestVars.mockKvWriter.ClearDs()
	// TODO: add more
}

func testAddDeleteService(t *testing.T) {
	svc := &coreV1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kubernetes",
			Namespace:       "default",
			SelfLink:        "/api/v1/namespaces/default/services/kubernetes",
			UID:             "8ca8bfdc-ec4c-11e7-9959-0800271d72be",
			ResourceVersion: "16",
			Generation:      0,
			CreationTimestamp: metav1.Date(2017, 12, 28, 19, 58, 37, 0,
				time.FixedZone("PST", -800)),
			Labels: map[string]string{"component: apiserver": "provider: kubernetes"},
		},
		Spec: coreV1.ServiceSpec{
			Ports: []coreV1.ServicePort{
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   0,
						IntVal: 6443,
					},
				},
			},
			Selector:  map[string]string{},
			ClusterIP: "10.96.0.1",
			Type:      "ClusterIP",
		},
	}
	// Check if we can add a service
	serviceTestVars.k8sListWatch.Add(svc)

	svcProto := &proto.Service{}
	err := serviceTestVars.mockKvWriter.GetValue(proto.Key(svc.GetName(), svc.GetNamespace()), svcProto)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(svcProto).NotTo(gomega.BeNil())
	gomega.Expect(svcProto.Name).To(gomega.Equal(svc.GetName()))
	gomega.Expect(svcProto.Namespace).To(gomega.Equal(svc.GetNamespace()))
	gomega.Expect(svcProto.ClusterIp).To(gomega.Equal(svc.Spec.ClusterIP))
	gomega.Expect(len(svcProto.Selector)).To(gomega.Equal(len(svc.Spec.Selector)))
	gomega.Expect(svcProto.ServiceType).To(gomega.Equal(string(svc.Spec.Type)))
	gomega.Expect(svcProto.LoadbalancerIp).To(gomega.Equal(svc.Spec.LoadBalancerIP))
	gomega.Expect(len(svcProto.Port)).Should(gomega.BeNumerically("==", 1))
	gomega.Expect(svcProto.Port[0].Name).To(gomega.Equal(svc.Spec.Ports[0].Name))
	gomega.Expect(svcProto.Port[0].Port).To(gomega.Equal(svc.Spec.Ports[0].Port))
	gomega.Expect(svcProto.Port[0].Protocol).To(gomega.Equal(string(svc.Spec.Ports[0].Protocol)))
	gomega.Expect(svcProto.Port[0].TargetPort.Type).To(gomega.BeEquivalentTo(svc.Spec.Ports[0].TargetPort.Type))
	gomega.Expect(svcProto.Port[0].TargetPort.IntVal).To(gomega.BeEquivalentTo(string(svc.Spec.Ports[0].TargetPort.IntVal)))

	// Now check if we can delete the newly added service
	serviceTestVars.k8sListWatch.Delete(svc)
	svcProto = &proto.Service{}
	key := proto.Key(svc.GetName(), svc.GetNamespace())
	err = serviceTestVars.mockKvWriter.GetValue(key, svcProto)
	gomega.Ω(err).ShouldNot(gomega.Succeed())
}
