/*
Copyright 2023 Red Hat, Inc.

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

package providerconfig

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-api-actuator-pkg/testutils"
	configv1resourcebuilder "github.com/openshift/cluster-api-actuator-pkg/testutils/resourcebuilder/config/v1"
	machinev1resourcebuilder "github.com/openshift/cluster-api-actuator-pkg/testutils/resourcebuilder/machine/v1"
	machinev1beta1resourcebuilder "github.com/openshift/cluster-api-actuator-pkg/testutils/resourcebuilder/machine/v1beta1"
)

var _ = Describe("VSphere Provider Config", Label("vSphereProviderConfig"), func() {
	var logger testutils.TestLogger

	var providerConfig VSphereProviderConfig

	usCentral1a := "us-central1-a"
	usCentral1b := "us-central1-b"

	logger = testutils.NewTestLogger()

	BeforeEach(func() {
		machineProviderConfig := machinev1beta1resourcebuilder.VSphereProviderSpec().
			WithZone(usCentral1a).
			Build()

		providerConfig = VSphereProviderConfig{
			providerConfig: *machineProviderConfig,
			infrastructure: configv1resourcebuilder.Infrastructure().AsVSphereWithFailureDomains("vsphere-test", nil).Build(),
		}
	})

	Context("ExtractFailureDomain", func() {
		It("returns the configured failure domain", func() {
			expected := machinev1resourcebuilder.VSphereFailureDomain().
				WithZone(usCentral1a).
				Build()

			Expect(providerConfig.ExtractFailureDomain()).To(Equal(expected))
		})
	})

	Context("when the failuredomain is vm-host zonal", func() {
		BeforeEach(func() {
			infrastructure := configv1resourcebuilder.Infrastructure().AsVSphereWithFailureDomains("vsphere-test", nil).WithVSphereVMHostZonal().Build()

			machineProviderConfig := machinev1beta1resourcebuilder.VSphereProviderSpec().
				WithInfrastructure(*infrastructure).
				WithZone(usCentral1a).
				Build()

			providerConfig = VSphereProviderConfig{
				providerConfig: *machineProviderConfig,
				infrastructure: infrastructure,
			}

		})
		Context("ExtractFailureDomain", func() {
			It("returns the configured failure domain", func() {
				expected := machinev1resourcebuilder.VSphereFailureDomain().
					WithZone(usCentral1a).
					Build()

				Expect(providerConfig.ExtractFailureDomain()).To(Equal(expected))

			})
		})
	})

	Context("when the failuredomain is changed after initialisation", func() {
		var changedProviderConfig VSphereProviderConfig

		BeforeEach(func() {
			changedFailureDomain := machinev1resourcebuilder.VSphereFailureDomain().
				WithZone(usCentral1b).
				Build()

			var err error
			changedProviderConfig, err = providerConfig.InjectFailureDomain(changedFailureDomain)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("ExtractFailureDomain", func() {
			It("returns the changed failure domain from the changed config", func() {
				expected := machinev1resourcebuilder.VSphereFailureDomain().
					WithZone(usCentral1b).
					Build()

				Expect(changedProviderConfig.ExtractFailureDomain()).To(Equal(expected))
			})

			It("returns the original failure domain from the original config", func() {
				expected := machinev1resourcebuilder.VSphereFailureDomain().
					WithZone(usCentral1a).
					Build()

				Expect(providerConfig.ExtractFailureDomain()).To(Equal(expected))
			})
		})
	})

	Context("StaticIP", func() {

		BeforeEach(func() {
			machineProviderConfig := machinev1beta1resourcebuilder.VSphereProviderSpec().
				WithZone(usCentral1a).WithIPPool().
				Build()

			providerConfig = VSphereProviderConfig{
				providerConfig: *machineProviderConfig,
				infrastructure: configv1resourcebuilder.Infrastructure().AsVSphereWithFailureDomains("vsphere-test", nil).Build(),
			}
		})

		It("contains an AddressesFromPools block", func() {
			Expect(providerConfig.providerConfig.Network.Devices[0].AddressesFromPools).To(Not(BeEmpty()),
				"expected AddressesFromPools to not be empty as a static IPPool has been configured")
		})

		It("returns networking with AddressesFromPools and the configured network name from failure domain", func() {
			expected, err := providerConfig.InjectFailureDomain(providerConfig.ExtractFailureDomain())
			Expect(err).To(Not(HaveOccurred()))
			Expect(expected.providerConfig.Network.Devices[0].AddressesFromPools).To(Not(BeNil()),
				"expected AddressesFromPools to still be present after injecting Failure Domain")
			Expect(expected.providerConfig.Network.Devices[0].NetworkName).To(Equal(providerConfig.infrastructure.Spec.PlatformSpec.VSphere.FailureDomains[0].Topology.Networks[0]),
				"expected NetworkName to still be equal to the the original after injection of the Failure Domain")
		})
	})

	Context("newVSphereProviderConfig", func() {
		var providerConfig ProviderConfig
		var expectedVSphereConfig machinev1beta1.VSphereMachineProviderSpec

		BeforeEach(func() {
			configBuilder := machinev1beta1resourcebuilder.VSphereProviderSpec()
			expectedVSphereConfig = *configBuilder.Build()
			rawConfig := configBuilder.BuildRawExtension()

			infrastructure := configv1resourcebuilder.Infrastructure().AsVSphere("vsphere-test").Build()
			var err error
			providerConfig, err = newVSphereProviderConfig(logger.Logger(), rawConfig, infrastructure)
			Expect(err).ToNot(HaveOccurred())
		})

		It("sets the type to VSphere", func() {
			Expect(providerConfig.Type()).To(Equal(configv1.VSpherePlatformType))
		})

		It("returns the correct VSphere config", func() {
			Expect(providerConfig.VSphere()).ToNot(BeNil())
			Expect(providerConfig.VSphere().Config()).To(Equal(expectedVSphereConfig))
		})
	})

	Context("with manual static ips", func() {
		var expectedFailureDomain v1.VSphereFailureDomain

		BeforeEach(func() {
			providerConfig.providerConfig.Network = machinev1beta1.NetworkSpec{
				Devices: []machinev1beta1.NetworkDeviceSpec{
					{
						IPAddrs: []string{
							"192.168.133.240",
						},
						Gateway: "192.168.133.1",
						Nameservers: []string{
							"8.8.8.8",
						},
						NetworkName: "test-network",
					},
				},
			}
			expectedFailureDomain = machinev1resourcebuilder.VSphereFailureDomain().
				WithZone(usCentral1a).
				Build()

		})

		It("returns the configured failure domain", func() {
			Expect(providerConfig.ExtractFailureDomain()).To(Equal(expectedFailureDomain))
		})

		It("returns expected provider config after injection", func() {
			injectedProviderConfig, err := providerConfig.InjectFailureDomain(expectedFailureDomain)
			Expect(err).ToNot(HaveOccurred())
			Expect(injectedProviderConfig.providerConfig.Network.Devices[0].IPAddrs).To(BeNil())
			Expect(injectedProviderConfig.providerConfig.Network.Devices[0].Gateway).To(Equal(""))
			Expect(injectedProviderConfig.providerConfig.Network.Devices[0].Nameservers).To(BeNil())
		})

		It("returns expected provider config after newVSphereProviderConfig", func() {
			configBuilder := machinev1beta1resourcebuilder.VSphereProviderSpec()

			machine := machinev1beta1resourcebuilder.Machine().AsMaster().WithProviderSpecBuilder(configBuilder).Build()

			vsMachine := &machinev1beta1.VSphereMachineProviderSpec{}
			err := json.Unmarshal(machine.Spec.ProviderSpec.Value.Raw, vsMachine)
			Expect(err).ToNot(HaveOccurred())

			vsMachine.Network.Devices[0].IPAddrs = []string{"192.168.133.14"}
			vsMachine.Network.Devices[0].Gateway = "192.168.133.1"
			vsMachine.Network.Devices[0].Nameservers = []string{"8.8.8.8"}
			vsMachine.Network.Devices[0].NetworkName = "test-network"

			jsonRaw, _ := json.Marshal(vsMachine)
			machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: jsonRaw}

			infrastructure := configv1resourcebuilder.Infrastructure().AsVSphere("vsphere-test").Build()
			newProviderSpec, err := newVSphereProviderConfig(logger.Logger(), machine.Spec.ProviderSpec.Value, infrastructure)

			Expect(err).ToNot(HaveOccurred())
			Expect(newProviderSpec.VSphere().providerConfig.Network.Devices[0].IPAddrs).To(BeNil())
			Expect(newProviderSpec.VSphere().providerConfig.Network.Devices[0].Gateway).To(Equal(""))
			Expect(newProviderSpec.VSphere().providerConfig.Network.Devices[0].Nameservers).To(BeNil())
		})
	})

	Context("no network configured", func() {
		BeforeEach(func() {
			providerConfig.providerConfig.Network = machinev1beta1.NetworkSpec{}
		})

		It("should not fail after injectFailureDomain", func() {
			expected, err := providerConfig.InjectFailureDomain(providerConfig.ExtractFailureDomain())
			Expect(err).To(Not(HaveOccurred()))
			Expect(expected.providerConfig.Network.Devices[0].NetworkName).To(Equal(providerConfig.infrastructure.Spec.PlatformSpec.VSphere.FailureDomains[0].Topology.Networks[0]),
				"expected NetworkName to still be equal to the the original after injection of the Failure Domain")
		})
	})

	Context("no vsphere platform spec in infrastructure", func() {
		BeforeEach(func() {
			providerConfig.infrastructure.Spec.PlatformSpec.VSphere = nil
		})

		It("should should return empty failure domain", func() {
			expected := providerConfig.ExtractFailureDomain()

			Expect(expected).To(Equal(v1.VSphereFailureDomain{}))
		})
	})

	// OCPBUGS-73867: when the datacenter path starts with a slash (e.g. "/nested-dc01"),
	// InjectFailureDomain must not produce a double-slash Folder, and ExtractFailureDomain
	// must find the failure domain regardless of whether the stored datacenter has the
	// leading slash or not.
	Context("when vCenter datacenter path begins with a leading slash", func() {
		const infraName = "vsphere-test"

		var leadingSlashInfra *configv1.Infrastructure

		BeforeEach(func() {
			leadingSlashInfra = configv1resourcebuilder.Infrastructure().AsVSphereWithFailureDomains(infraName, &[]configv1.VSpherePlatformFailureDomainSpec{
				{
					Name:   "us-central1-a",
					Region: "us-central",
					Zone:   "1-a",
					Server: "vcenter.test.com",
					Topology: configv1.VSpherePlatformTopology{
						Datacenter:     "/nested-dc01",
						ComputeCluster: "/nested-dc01/host/test-cluster-1",
						Networks:       []string{"test-network-1"},
						Datastore:      "/nested-dc01/datastore/test-datastore-1",
						ResourcePool:   "/nested-dc01/host/test-cluster-1/Resources",
					},
				},
			}).Build()
		})

		Context("InjectFailureDomain", func() {
			It("produces a Folder without a double slash", func() {
				cfg := VSphereProviderConfig{
					providerConfig: machinev1beta1.VSphereMachineProviderSpec{
						Workspace: &machinev1beta1.Workspace{
							Server:       "vcenter.test.com",
							Datacenter:   "/nested-dc01",
							Datastore:    "/nested-dc01/datastore/test-datastore-1",
							ResourcePool: "/nested-dc01/host/test-cluster-1/Resources",
							Folder:       "/nested-dc01/vm/" + infraName,
						},
					},
					infrastructure: leadingSlashInfra,
					logger:         logger.Logger(),
				}

				fd := v1.VSphereFailureDomain{Name: "us-central1-a"}
				result, err := cfg.InjectFailureDomain(fd)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.providerConfig.Workspace.Folder).To(Equal("/nested-dc01/vm/"+infraName),
					"Folder must not contain a double slash when Datacenter starts with /")
			})

			It("produces no spurious Diff against an existing machine with a single-slash Folder", func() {
				// Template computed by InjectFailureDomain when no topology.Template is set:
				// "<infraName>-rhcos-<region>-<zone>"
				expectedTemplate := infraName + "-rhcos-us-central-1-a"

				// Simulates the state stored by the installer: datacenter with leading slash,
				// folder correctly formed with a single slash.
				existingSpec := machinev1beta1.VSphereMachineProviderSpec{
					Workspace: &machinev1beta1.Workspace{
						Server:       "vcenter.test.com",
						Datacenter:   "/nested-dc01",
						Datastore:    "/nested-dc01/datastore/test-datastore-1",
						ResourcePool: "/nested-dc01/host/test-cluster-1/Resources",
						Folder:       "/nested-dc01/vm/" + infraName,
					},
					Network: machinev1beta1.NetworkSpec{
						Devices: []machinev1beta1.NetworkDeviceSpec{
							{NetworkName: "test-network-1"},
						},
					},
					Template: expectedTemplate,
				}

				cfg := VSphereProviderConfig{
					providerConfig: existingSpec,
					infrastructure: leadingSlashInfra,
					logger:         logger.Logger(),
				}

				fd := v1.VSphereFailureDomain{Name: "us-central1-a"}
				desired, err := cfg.InjectFailureDomain(fd)
				Expect(err).ToNot(HaveOccurred())

				diffs, err := cfg.Diff(desired.providerConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(diffs).To(BeEmpty(),
					"Diff must be empty: leading-slash Datacenter must not trigger a spurious rollout")
			})
		})

		Context("ExtractFailureDomain", func() {
			It("matches when the machine workspace has the datacenter without a leading slash", func() {
				// Some installs store the datacenter without the slash even when install-config
				// used one. ExtractFailureDomain must still find the right failure domain.
				cfg := VSphereProviderConfig{
					providerConfig: machinev1beta1.VSphereMachineProviderSpec{
						Workspace: &machinev1beta1.Workspace{
							Server:       "vcenter.test.com",
							Datacenter:   "nested-dc01", // no leading slash
							Datastore:    "/nested-dc01/datastore/test-datastore-1",
							ResourcePool: "/nested-dc01/host/test-cluster-1/Resources",
						},
					},
					infrastructure: leadingSlashInfra,
					logger:         logger.Logger(),
				}

				result := cfg.ExtractFailureDomain()
				Expect(result).To(Equal(v1.VSphereFailureDomain{Name: "us-central1-a"}),
					"ExtractFailureDomain must match even when stored Datacenter lacks the leading slash")
			})

			It("matches when the machine workspace has the datacenter with a leading slash", func() {
				cfg := VSphereProviderConfig{
					providerConfig: machinev1beta1.VSphereMachineProviderSpec{
						Workspace: &machinev1beta1.Workspace{
							Server:       "vcenter.test.com",
							Datacenter:   "/nested-dc01", // with leading slash
							Datastore:    "/nested-dc01/datastore/test-datastore-1",
							ResourcePool: "/nested-dc01/host/test-cluster-1/Resources",
						},
					},
					infrastructure: leadingSlashInfra,
					logger:         logger.Logger(),
				}

				result := cfg.ExtractFailureDomain()
				Expect(result).To(Equal(v1.VSphereFailureDomain{Name: "us-central1-a"}),
					"ExtractFailureDomain must match when stored Datacenter has a leading slash")
			})
		})
	})
})
