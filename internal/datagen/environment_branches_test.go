package datagen

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplianceForWithoutTheVendorIsNil pins that ApplianceFor returns nil
// when the environment has no device of the vendor.
func TestApplianceForWithoutTheVendorIsNil(t *testing.T) {
	e := &Environment{NetworkSystems: []*NetworkSystemIdentity{{Hostname: "cat-sw-01", Vendor: VendorCisco}}}
	require.Nil(t, e.ApplianceFor("palo-alto", 0, VendorPaloAlto))
	require.Equal(t, "cat-sw-01", e.ApplianceFor("x", 0, VendorCisco).Hostname)
}

// TestNetworkSystemIdentityForUnmodeledVendorFallsBack pins that a vendor
// with no modeled hardware still gets a valid device.
func TestNetworkSystemIdentityForUnmodeledVendorFallsBack(t *testing.T) {
	n := NetworkSystemIdentityFor(rand.New(rand.NewSource(1)), ApplianceVendor("Acme")) // #nosec G404
	require.NotNil(t, n)
	require.NoError(t, n.Validate())
}

// TestSpecForDefaultsAndPools pins specFor where one fleet spec fits (that
// spec) or none does (req's OS and first role, a workstation for macOS by
// default; DCs draw Greek names, routers Egyptian, and the rest Roman).
func TestSpecForDefaultsAndPools(t *testing.T) {
	for _, tc := range []struct {
		req  HostRequirement
		want osRoleSpec
	}{
		{HostRequirement{Roles: []SystemRole{RoleDC}}, osRoleSpec{OSWindows, RoleDC, GreekNames}},
		{HostRequirement{Roles: []SystemRole{RoleWorkstation}}, osRoleSpec{OSWindows, RoleWorkstation, RomanNames}},
		{HostRequirement{OS: OSMacOS}, osRoleSpec{OSMacOS, RoleWorkstation, RomanNames}},
		{HostRequirement{Roles: []SystemRole{RoleRouter}}, osRoleSpec{OSLinux, RoleRouter, EgyptianNames}},
		{HostRequirement{OS: OSLinux, Roles: []SystemRole{RoleWorkstation}}, osRoleSpec{OSLinux, RoleWorkstation, RomanNames}},
		{HostRequirement{OS: OSLinux, Roles: []SystemRole{RoleDC}}, osRoleSpec{OSLinux, RoleDC, GreekNames}},
		{HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleRouter}}, osRoleSpec{OSWindows, RoleRouter, EgyptianNames}},
	} {
		require.Equal(t, tc.want, specFor(rand.New(rand.NewSource(1)), tc.req), "%+v", tc.req) // #nosec G404
	}
}

// TestSpecForWeighsAllowedRoles pins that a host appended for a requirement
// allowing several roles (wel with no role) draws its role from the fleet's
// role weights limited to those roles: member servers most, then
// workstations, then domain controllers.
func TestSpecForWeighsAllowedRoles(t *testing.T) {
	r := rand.New(rand.NewSource(1)) // #nosec G404
	req := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC, RoleServer, RoleWorkstation}}
	counts := map[SystemRole]int{}
	for range 3000 {
		spec := specFor(r, req)
		require.Equal(t, OSWindows, spec.os)
		counts[spec.role]++
	}
	require.Greater(t, counts[RoleServer], counts[RoleWorkstation])
	require.Greater(t, counts[RoleWorkstation], counts[RoleDC])
	require.Positive(t, counts[RoleDC])
}
