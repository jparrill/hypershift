package util

import (
	"reflect"
	"testing"
	"unicode/utf8"

	. "github.com/onsi/gomega"
	"github.com/openshift/hypershift/api/util/ipnet"
	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
)

func TestCompressDecompress(t *testing.T) {
	testCases := []struct {
		name       string
		payload    []byte
		compressed []byte
	}{
		{
			name:       "Text",
			payload:    []byte("The quick brown fox jumps over the lazy dog."),
			compressed: []byte("H4sIAAAAAAAC/wrJSFUoLM1MzlZIKsovz1NIy69QyCrNLShWyC9LLVIoyUhVyEmsqlRIyU/XAwQAAP//6SWQUSwAAAA="),
		},
		{
			name:       "Empty",
			payload:    []byte{},
			compressed: []byte{},
		},
		{
			name:       "Nil",
			payload:    nil,
			compressed: nil,
		},
	}

	t.Parallel()

	for _, tc := range testCases {
		t.Run(tc.name+" Valid Input", func(t *testing.T) {
			testCompressFunc(t, tc.payload, tc.compressed)
			testDecompressFunc(t, tc.payload, tc.compressed)
		})

		// Empty or nil inputs will not produce an error, which is what this test
		// looks for. Instead, they will produce a nil or empty result within
		// their initialized bytes.Buffer object.
		if len(tc.payload) != 0 {
			t.Run(tc.name+" Invalid Input", func(t *testing.T) {
				testDecompressFuncErr(t, tc.payload)
			})
		}
	}
}

// Tests that a given input can be expected and encoded without errors.
func testCompressFunc(t *testing.T, payload, expected []byte) {
	t.Helper()

	g := NewWithT(t)

	result, err := CompressAndEncode(payload)
	g.Expect(err).To(BeNil(), "should be no compression errors")
	g.Expect(result).ToNot(BeNil(), "should always return an initialized bytes.Buffer")

	resultBytes := result.Bytes()
	resultString := result.String()

	g.Expect(utf8.Valid(resultBytes)).To(BeTrue(), "expected output should be a valid utf-8 sequence")
	g.Expect(resultBytes).To(Equal(expected), "expected bytes should equal expected")
	g.Expect(resultString).To(Equal(string(expected)), "expected strings should equal expected")
}

// Tests that a given output can be decoded and decompressed without errors.
func testDecompressFunc(t *testing.T, payload, expected []byte) {
	t.Helper()

	g := NewWithT(t)

	result, err := DecodeAndDecompress(expected)
	g.Expect(err).To(BeNil(), "should be no decompression errors")
	g.Expect(result).ToNot(BeNil(), "should always return an initialized bytes.Buffer")

	resultBytes := result.Bytes()
	resultString := result.String()

	g.Expect(resultBytes).To(Equal(payload), "deexpected bytes should equal expected")
	g.Expect(resultString).To(Equal(string(payload)), "deexpected string should equal expected")
}

// Tests that an invalid decompression input (not gzipped and base64-encoded)
// will produce an error.
func testDecompressFuncErr(t *testing.T, payload []byte) {
	out, err := DecodeAndDecompress(payload)

	g := NewWithT(t)
	g.Expect(err).ToNot(BeNil(), "should be an error")
	g.Expect(out).ToNot(BeNil(), "should return an initialized bytes.Buffer")
	g.Expect(out.Bytes()).To(BeNil(), "should be a nil byte slice")
	g.Expect(out.String()).To(BeEmpty(), "should be an empty string")
}

func TestIsIPv4(t *testing.T) {
	type args struct {
		cidrs []string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "When an ipv4 CIDR is checked by isIPv4, it should return true",
			args: args{
				cidrs: []string{"192.168.1.35/24", "0.0.0.0/0", "127.0.0.1/24"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "When an ipv6 CIDR is checked by isIPv4, it should return false",
			args: args{
				cidrs: []string{"2001::/17", "2001:db8::/62", "::/0", "2000::/3"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "When a non valid CIDR is checked by isIPv4, it should return an error and false",
			args: args{
				cidrs: []string{"192.168.35/68"},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, cidr := range tt.args.cidrs {
				got, err := IsIPv4(cidr)
				if (err != nil) != tt.wantErr {
					t.Errorf("isIPv4() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("isIPv4() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGetDefaultIpsForSvcNets(t *testing.T) {
	const (
		DefaultAdvertiseIPv4Address = "172.20.0.1"
		DefaultAdvertiseIPv6Address = "fd02::1"
	)
	type args struct {
		cidrs    []string
		defaults *DefaultAdvIps
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Given IPv6 CIDRs, it should only return fd02::1",
			args: args{
				cidrs: []string{"2001::/17", "2001:db8::/62", "::/0", "2000::/3"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv6Address},
		},
		{
			name: "Given IPv4 CIDRs, it should only return 172.20.0.1",
			args: args{
				cidrs: []string{"192.168.1.35/24", "0.0.0.0/0", "127.0.0.1/24"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv4Address},
		},
		{
			name: "Given IPv6 and IPv4 CIDRs, it should return both fd02::1 and 172.20.0.1",
			args: args{
				cidrs: []string{"2001::/17", "192.168.1.35/24", "::/0", "2000::/3"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv6Address, DefaultAdvertiseIPv4Address},
		},
		{
			name: "Given IPv6 and malformed IPv4 CIDRs, it should return fd02::1",
			args: args{
				cidrs: []string{"2001::/17", "192.168.1.35.53/24", "::/0", "2000::/3"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv6Address},
		},
		{
			name: "Given IPv4 and malformed IPv6 CIDRs, it should return 172.20.0.1",
			args: args{
				cidrs: []string{"2001::44444444444444/17", "192.168.1.35/24"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv4Address},
		},
		{
			name: "Given malformed IPv4 and malformed IPv6 CIDRs, it should return 172.20.0.1",
			args: args{
				cidrs: []string{"2001::44444444444444/17", "192.168.1.3.47/24"},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv4Address},
		},
		{
			name: "Given an empty slice of CIDRs, it should return 172.20.0.1",
			args: args{
				cidrs: []string{},
				defaults: &DefaultAdvIps{
					IPv4: DefaultAdvertiseIPv4Address,
					IPv6: DefaultAdvertiseIPv6Address,
				},
			},
			want: []string{DefaultAdvertiseIPv4Address},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceNets := make([]hyperv1.ServiceNetworkEntry, len(tt.args.cidrs))
			for i, cidr := range tt.args.cidrs {
				ipNetCidr, err := ipnet.ParseCIDR(cidr)
				if err != nil {
					continue
				}
				serviceNets[i].CIDR = *ipNetCidr
			}
			if got := GetDefaultIpsForSvcNets(serviceNets, tt.args.defaults); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDefaultIpsForSvcNets() = %v, want %v", got, tt.want)
			}
		})
	}
}
