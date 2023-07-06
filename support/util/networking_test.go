package util

import (
	"reflect"
	"testing"

	"github.com/openshift/hypershift/api/util/ipnet"
	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
	"k8s.io/utils/pointer"
)

func TestSetAdvertiseAddress(t *testing.T) {
	type args struct {
		hcp      *hyperv1.HostedControlPlane
		defaults *DefaultAdvIps
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "given an AdvertiseAddress in the HCP, it should return it",
			args: args{
				hcp: &hyperv1.HostedControlPlane{
					Spec: hyperv1.HostedControlPlaneSpec{
						Networking: hyperv1.ClusterNetworking{
							APIServer: &hyperv1.APIServerNetworking{
								AdvertiseAddress: pointer.String("192.168.1.1"),
							},
						},
					},
				},
				defaults: &DefaultAdvIps{
					IPv4: "172.20.0.1",
					IPv6: "fd00::1",
				},
			},
			want: "192.168.1.1",
		},
		{
			name: "given no AdvertiseAddress/es in the HCP, it should return IPv4 based default address",
			args: args{
				hcp: &hyperv1.HostedControlPlane{
					Spec: hyperv1.HostedControlPlaneSpec{
						Networking: hyperv1.ClusterNetworking{
							ServiceNetwork: []hyperv1.ServiceNetworkEntry{{
								CIDR: *ipnet.MustParseCIDR("192.168.1.0/24"),
							}},
						},
					},
				},
				defaults: &DefaultAdvIps{
					IPv4: "172.20.0.1",
					IPv6: "fd00::1",
				},
			},
			want: "172.20.0.1",
		},
		{
			name: "given no AdvertiseAddress/es in the HCP, it should return IPv6 based default address",
			args: args{
				hcp: &hyperv1.HostedControlPlane{
					Spec: hyperv1.HostedControlPlaneSpec{
						Networking: hyperv1.ClusterNetworking{
							ServiceNetwork: []hyperv1.ServiceNetworkEntry{{
								CIDR: *ipnet.MustParseCIDR("2620:52:0:1306::1/64"),
							}},
						},
					},
				},
				defaults: &DefaultAdvIps{
					IPv4: "172.20.0.1",
					IPv6: "fd00::1",
				},
			},
			want: "fd00::1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetAdvertiseAddress(tt.args.hcp, tt.args.defaults); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetAdvertiseAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
