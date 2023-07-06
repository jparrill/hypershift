package pki

import (
	"fmt"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"

	"github.com/openshift/hypershift/support/config"
	"github.com/openshift/hypershift/support/globalconfig"
	"github.com/openshift/hypershift/support/util"
)

type PKIParams struct {
	// ServiceCIDR
	// Subnet for cluster services
	ServiceCIDR string `json:"serviceCIDR"`

	// ClusterCIDR
	// Subnet for pods
	ClusterCIDR string `json:"clusterCIDR"`

	// ExternalAPIAddress
	// An externally accessible DNS name or IP for the API server. Currently obtained from the load balancer DNS name.
	ExternalAPIAddress string `json:"externalAPIAddress"`

	// InternalAPIAddress
	// An internally accessible DNS name or IP for the API server.
	InternalAPIAddress string `json:"internalAPIAddress"`

	// ExternalKconnectivityAddress
	// An externally accessible DNS name or IP for the Konnectivity proxy. Currently obtained from the load balancer DNS name.
	ExternalKconnectivityAddress string `json:"externalKconnectivityAddress"`

	// NodeInternalAPIServerIP
	// A fixed IP that pods on worker nodes will use to communicate with the API server - 172.20.0.1 for IPv4 and fd02::1 in IPv6 case
	NodeInternalAPIServerIP string `json:"nodeInternalAPIServerIP"`

	// ExternalOauthAddress
	// An externally accessible DNS name or IP for the Oauth server. Currently obtained from Oauth load balancer DNS name.
	ExternalOauthAddress string `json:"externalOauthAddress"`

	// IngressSubdomain
	// Subdomain for cluster ingress. Used to generate the wildcard certificate for ingress.
	IngressSubdomain string `json:"ingressSubdomain"`

	// Namespace used to generate internal DNS names for services.
	Namespace string `json:"namespace"`

	// Owner reference for resources
	OwnerRef config.OwnerRef `json:"ownerRef"`
}

func NewPKIParams(hcp *hyperv1.HostedControlPlane,
	apiExternalAddress,
	oauthExternalAddress,
	konnectivityExternalAddress string) *PKIParams {
	p := &PKIParams{
		ServiceCIDR:                  util.FirstServiceCIDR(hcp.Spec.Networking.ServiceNetwork),
		ClusterCIDR:                  util.FirstClusterCIDR(hcp.Spec.Networking.ClusterNetwork),
		Namespace:                    hcp.Namespace,
		ExternalAPIAddress:           apiExternalAddress,
		InternalAPIAddress:           fmt.Sprintf("api.%s.hypershift.local", hcp.Name),
		ExternalKconnectivityAddress: konnectivityExternalAddress,
		ExternalOauthAddress:         oauthExternalAddress,
		IngressSubdomain:             globalconfig.IngressDomain(hcp),
		OwnerRef:                     config.OwnerRefFrom(hcp),
	}

	// ToDo (jparrill): When we support more than 1 ServiceNetwork for PKI we need to move from using
	// util.AdvertiseAddressWithDefault to util.SetAdvertiseAddresses where we cover dual stack stuff

	ipv4, err := util.IsIPv4(p.ServiceCIDR)
	if err != nil {
		fmt.Printf("error checking the ServiceNetworkCIDRs: %v", err)
	}

	// Set the default
	if ipv4 {
		p.NodeInternalAPIServerIP = util.AdvertiseAddressWithDefault(hcp, config.DefaultAdvertiseIPv4Address)
	} else {
		p.NodeInternalAPIServerIP = util.AdvertiseAddressWithDefault(hcp, config.DefaultAdvertiseIPv6Address)
	}
	return p
}
