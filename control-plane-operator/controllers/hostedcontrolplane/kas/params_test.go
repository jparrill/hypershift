package kas

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
	"github.com/openshift/hypershift/support/config"
)

// TODO (cewong): Add tests for other params
func TestNewAPIServerParamsAPIAdvertiseAddressAndPort(t *testing.T) {
	tests := []struct {
		apiServiceMapping hyperv1.ServicePublishingStrategyMapping
		name              string
		advertiseAddress  *string
		port              *int32
		expectedAddress   string
		expectedPort      int32
	}{
		{
			name:            "not specified",
			expectedAddress: config.DefaultAdvertiseIPv4Address,
			expectedPort:    config.DefaultAPIServerPort,
		},
		{
			name:             "address specified",
			advertiseAddress: pointer.StringPtr("1.2.3.4"),
			expectedAddress:  "1.2.3.4",
			expectedPort:     config.DefaultAPIServerPort,
		},
		{
			name:            "port set for default service publishing strategies",
			port:            pointer.Int32(6789),
			expectedAddress: config.DefaultAdvertiseIPv4Address,
			expectedPort:    config.DefaultAPIServerPort,
		},
		{
			name: "port set for NodePort service Publishing Strategy",
			apiServiceMapping: hyperv1.ServicePublishingStrategyMapping{
				Service: hyperv1.APIServer,
				ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
					Type: hyperv1.NodePort,
				},
			},
			port:            pointer.Int32(6789),
			expectedAddress: config.DefaultAdvertiseIPv4Address,
			expectedPort:    6789,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hcp := &hyperv1.HostedControlPlane{}
			hcp.Spec.Services = []hyperv1.ServicePublishingStrategyMapping{test.apiServiceMapping}
			hcp.Spec.Networking.APIServer = &hyperv1.APIServerNetworking{Port: test.port, AdvertiseAddress: test.advertiseAddress}
			p := NewKubeAPIServerParams(context.Background(), hcp, map[string]string{}, "", 0, "", 0, false)
			g := NewGomegaWithT(t)
			g.Expect(p.AdvertiseAddress).To(Equal(test.expectedAddress))
			g.Expect(p.APIServerPort).To(Equal(test.expectedPort))
		})
	}
}
