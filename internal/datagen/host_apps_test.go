package datagen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAppsRequirementNeedsApplications pins that a requirement for server
// software (Apps) accepts only hosts that run applications, and does not
// overlap a requirement whose OS runs none in the roles both allow, so the
// environment provisions a separate host for each.
func TestAppsRequirementNeedsApplications(t *testing.T) {
	software := HostRequirement{Roles: []SystemRole{RoleServer}, Apps: true}
	macServer := &SystemIdentity{Role: RoleServer, OSInfo: OSInfo{Type: OSMacOS}}
	linuxServer := &SystemIdentity{Role: RoleServer, OSInfo: OSInfo{Type: OSLinux}}

	assert.False(t, software.accepts(macServer), "a server that runs no applications")
	assert.True(t, software.accepts(linuxServer))
	assert.True(t, HostRequirement{Roles: []SystemRole{RoleServer}}.accepts(macServer), "without Apps any server")

	assert.False(t, software.overlaps(HostRequirement{OS: OSMacOS}))
	assert.False(t, HostRequirement{OS: OSMacOS}.overlaps(software))
	assert.True(t, software.overlaps(HostRequirement{OS: OSLinux}))
	assert.True(t, software.overlaps(HostRequirement{}))
}

// TestOverlapsByRole pins overlaps' role rules: requirements whose role sets
// are disjoint never overlap, shared roles do, and an Apps requirement with no
// roles overlaps when any role of the OS runs applications.
func TestOverlapsByRole(t *testing.T) {
	dc := HostRequirement{Roles: []SystemRole{RoleDC}}
	assert.False(t, dc.overlaps(HostRequirement{Roles: []SystemRole{RoleWorkstation}}))
	assert.True(t, HostRequirement{Roles: []SystemRole{RoleDC, RoleServer}}.overlaps(HostRequirement{Roles: []SystemRole{RoleServer}}))
	assert.True(t, HostRequirement{Apps: true}.overlaps(HostRequirement{OS: OSLinux}))
}
