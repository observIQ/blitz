package catalog

import "slices"

// Names are the F5 product names the generator registers. Config validation
// reads them here, in the leaf catalog package, so it need not import the
// generator; TestProductNamesMatchRegistry keeps the two in step.
var Names = []string{"asm", "afm", "apm", "dns", "audit", "nginx-plus", "nginx-app-protect", "irules", "f5os"}

// Known reports whether name is an F5 product.
func Known(name string) bool { return slices.Contains(Names, name) }
