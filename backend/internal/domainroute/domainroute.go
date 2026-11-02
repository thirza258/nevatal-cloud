// Package domainroute decides whether a Nevatal Domain can be routed to its
// instance. The API (the live_status of a domain) and nevatal-edge (whether
// to proxy a request) both call Policy.Evaluate, so the dashboard and the
// proxy never disagree.
package domainroute

import (
	"net/netip"
	"strings"

	"github.com/nevatal/cloud/internal/models"
)

// Status is the routing state of a domain.
type Status string

const (
	StatusLive            Status = "live"
	StatusDisabled        Status = "disabled"         // owner toggle off, disabled by an administrator, or owner suspended
	StatusInstanceMissing Status = "instance_missing" // no target instance, or it was deleted
	StatusInstanceStopped Status = "instance_stopped" // instance exists but is not running
	StatusNoIP            Status = "no_ip"            // instance has no private address yet
	StatusIPNotRoutable   Status = "ip_not_routable"  // address outside EDGE_TARGET_CIDRS, denied or special-purpose
	StatusIPConflict      Status = "ip_conflict"      // another non-deleted instance reports the same address
)

// Policy says which addresses the edge may proxy to.
type Policy struct {
	TargetCIDRs []netip.Prefix // an address must be inside one of these (none = route nothing)
	DenyIPs     []netip.Addr   // and must not be one of these
}

// Target is what Evaluate needs to know about a domain and its instance.
type Target struct {
	Enabled        bool   // owner toggle
	AdminDisabled  bool   // set by an administrator
	OwnerActive    bool   // owner account is not suspended
	InstanceFound  bool   // target instance exists and is not deleted
	InstanceStatus string // instance status (models.State*)
	PrivateIP      string // instance private address as stored; "" when unknown
}

// Evaluate returns the routing status of t and, when the status is
// StatusLive, the address to dial. ipUsers counts the non-deleted instances
// per private address (CountAddrs): an address reported by more than one
// instance is never routed, because the edge cannot tell which VM owns it.
//
// The checks run in a fixed order: disabled, instance missing, instance not
// running, no address, address not allowed, address shared, live.
func (p Policy) Evaluate(t Target, ipUsers map[netip.Addr]int) (Status, netip.Addr) {
	switch {
	case !t.Enabled || t.AdminDisabled || !t.OwnerActive:
		return StatusDisabled, netip.Addr{}
	case !t.InstanceFound:
		return StatusInstanceMissing, netip.Addr{}
	case t.InstanceStatus != models.StateRunning:
		return StatusInstanceStopped, netip.Addr{}
	case strings.TrimSpace(t.PrivateIP) == "":
		return StatusNoIP, netip.Addr{}
	}
	addr, ok := ParseAddr(t.PrivateIP)
	if !ok || !p.Allows(addr) {
		return StatusIPNotRoutable, netip.Addr{}
	}
	if ipUsers[addr] > 1 {
		return StatusIPConflict, netip.Addr{}
	}
	return StatusLive, addr
}

// Allows reports whether the edge may open a connection to addr: a unicast
// address inside one of the target CIDRs that is not denied, loopback,
// link-local or unspecified.
func (p Policy) Allows(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.Zone() != "" || !IsUsableUnicast(addr) {
		return false
	}
	for _, denied := range p.DenyIPs {
		if denied.Unmap() == addr {
			return false
		}
	}
	for _, prefix := range p.TargetCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// IsUsableUnicast reports whether addr can be a VM's LAN address: not
// unspecified, loopback, link-local or multicast.
func IsUsableUnicast(addr netip.Addr) bool {
	return addr.IsValid() &&
		!addr.IsUnspecified() &&
		!addr.IsLoopback() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsLinkLocalMulticast() &&
		!addr.IsInterfaceLocalMulticast() &&
		!addr.IsMulticast()
}

// ParseAddr parses a stored instance address. IPv4-mapped IPv6 addresses are
// unmapped so that both spellings of an address compare equal.
func ParseAddr(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// CountAddrs counts how many instances report each private address.
// Unparsable entries are ignored (they are never routed anyway).
func CountAddrs(ips []string) map[netip.Addr]int {
	counts := make(map[netip.Addr]int, len(ips))
	for _, ip := range ips {
		if addr, ok := ParseAddr(ip); ok {
			counts[addr]++
		}
	}
	return counts
}
