package domainroute

import (
	"net/netip"
	"testing"
)

func TestEvaluate(t *testing.T) {
	policy := Policy{
		TargetCIDRs: []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24"), netip.MustParsePrefix("fd00:42::/64")},
		DenyIPs:     []netip.Addr{netip.MustParseAddr("10.10.42.1")},
	}
	live := Target{Enabled: true, OwnerActive: true, InstanceFound: true, InstanceStatus: "running", PrivateIP: "10.10.42.20"}
	users := CountAddrs([]string{"10.10.42.20", "10.10.42.30", "::ffff:10.10.42.30", "garbage"})

	with := func(mut func(*Target)) Target {
		t := live
		mut(&t)
		return t
	}
	cases := map[string]struct {
		target Target
		want   Status
	}{
		"live":                  {live, StatusLive},
		"live ipv6":             {with(func(t *Target) { t.PrivateIP = "fd00:42::5" }), StatusLive},
		"owner disabled":        {with(func(t *Target) { t.Enabled = false }), StatusDisabled},
		"admin disabled":        {with(func(t *Target) { t.AdminDisabled = true }), StatusDisabled},
		"owner suspended":       {with(func(t *Target) { t.OwnerActive = false }), StatusDisabled},
		"disabled wins":         {with(func(t *Target) { t.Enabled = false; t.InstanceFound = false }), StatusDisabled},
		"instance missing":      {with(func(t *Target) { t.InstanceFound = false }), StatusInstanceMissing},
		"stopped":               {with(func(t *Target) { t.InstanceStatus = "stopped" }), StatusInstanceStopped},
		"restarting":            {with(func(t *Target) { t.InstanceStatus = "restarting" }), StatusInstanceStopped},
		"no ip":                 {with(func(t *Target) { t.PrivateIP = " " }), StatusNoIP},
		"garbage ip":            {with(func(t *Target) { t.PrivateIP = "10.10.42" }), StatusIPNotRoutable},
		"outside cidr":          {with(func(t *Target) { t.PrivateIP = "172.17.0.2" }), StatusIPNotRoutable},
		"denied":                {with(func(t *Target) { t.PrivateIP = "10.10.42.1" }), StatusIPNotRoutable},
		"loopback":              {with(func(t *Target) { t.PrivateIP = "127.0.0.1" }), StatusIPNotRoutable},
		"link-local":            {with(func(t *Target) { t.PrivateIP = "169.254.10.10" }), StatusIPNotRoutable},
		"unspecified":           {with(func(t *Target) { t.PrivateIP = "0.0.0.0" }), StatusIPNotRoutable},
		"zoned":                 {with(func(t *Target) { t.PrivateIP = "fd00:42::5%eth0" }), StatusIPNotRoutable},
		"conflict (mapped too)": {with(func(t *Target) { t.PrivateIP = "10.10.42.30" }), StatusIPConflict},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, addr := policy.Evaluate(tc.target, users)
			if got != tc.want {
				t.Fatalf("Evaluate = %s, want %s", got, tc.want)
			}
			if (got == StatusLive) != addr.IsValid() {
				t.Fatalf("an address must be returned exactly when live, got %v", addr)
			}
		})
	}
}

func TestEmptyPolicyRoutesNothing(t *testing.T) {
	got, _ := Policy{}.Evaluate(Target{Enabled: true, OwnerActive: true, InstanceFound: true, InstanceStatus: "running", PrivateIP: "10.10.42.20"}, nil)
	if got != StatusIPNotRoutable {
		t.Fatalf("no EDGE_TARGET_CIDRS must route nothing, got %s", got)
	}
}
