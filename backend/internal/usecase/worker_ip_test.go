package usecase

import (
	"net/netip"
	"testing"

	"github.com/nevatal/cloud/internal/proxmox"
)

func iface(name string, ips ...string) proxmox.NetworkInterface {
	ni := proxmox.NetworkInterface{Name: name}
	for _, ip := range ips {
		typ := "ipv4"
		if netip.MustParseAddr(ip).Is6() {
			typ = "ipv6"
		}
		ni.IPAddresses = append(ni.IPAddresses, proxmox.IPInfo{IPAddressType: typ, IPAddress: ip})
	}
	return ni
}

func TestSelectGuestIPv4(t *testing.T) {
	lan := []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24")}
	cases := map[string]struct {
		ifaces    []proxmox.NetworkInterface
		preferred []netip.Prefix
		want      string
	}{
		"docker0 listed first is skipped": {
			ifaces:    []proxmox.NetworkInterface{iface("lo", "127.0.0.1"), iface("docker0", "172.17.0.1"), iface("eth0", "10.10.42.7", "fe80::1")},
			preferred: lan,
			want:      "10.10.42.7",
		},
		"prefers the edge network over another LAN": {
			ifaces:    []proxmox.NetworkInterface{iface("ens18", "192.168.1.20"), iface("ens19", "10.10.42.8")},
			preferred: lan,
			want:      "10.10.42.8",
		},
		"container and VPN interfaces skipped even inside the CIDR": {
			ifaces: []proxmox.NetworkInterface{
				iface("br-3f2a9c", "10.10.42.200"), iface("veth12ab", "10.10.42.201"), iface("virbr0", "10.10.42.202"),
				iface("cni0", "10.10.42.203"), iface("flannel.1", "10.10.42.204"), iface("cali123", "10.10.42.205"),
				iface("tailscale0", "10.10.42.206"), iface("wg0", "10.10.42.207"), iface("enp0s18", "10.10.42.9"),
			},
			preferred: lan,
			want:      "10.10.42.9",
		},
		"no CIDR match: first regular address": {
			ifaces:    []proxmox.NetworkInterface{iface("docker0", "172.17.0.1"), iface("eth0", "192.168.7.4"), iface("eth1", "192.168.8.4")},
			preferred: lan,
			want:      "192.168.7.4",
		},
		"no CIDRs configured": {
			ifaces: []proxmox.NetworkInterface{iface("docker0", "172.17.0.1"), iface("eth0", "10.10.42.7")},
			want:   "10.10.42.7",
		},
		"link-local skipped": {
			ifaces:    []proxmox.NetworkInterface{iface("eth0", "169.254.3.4"), iface("eth1", "192.168.7.4")},
			preferred: lan,
			want:      "192.168.7.4",
		},
		"falls back to the original rule": {
			ifaces: []proxmox.NetworkInterface{iface("lo", "127.0.0.1"), iface("docker0", "172.17.0.1")},
			want:   "172.17.0.1",
		},
		"original rule keeps a link-local address as last resort": {
			ifaces: []proxmox.NetworkInterface{iface("eth0", "169.254.3.4")},
			want:   "169.254.3.4",
		},
		"ipv6 only": {
			ifaces: []proxmox.NetworkInterface{iface("lo", "127.0.0.1", "::1"), iface("eth0", "fd00::7")},
			want:   "",
		},
		"nothing": {want: ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := selectGuestIPv4(tc.ifaces, tc.preferred); got != tc.want {
				t.Fatalf("selectGuestIPv4 = %q, want %q", got, tc.want)
			}
		})
	}
}
