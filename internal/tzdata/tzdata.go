// Package tzdata supplies the Go timezone database to local runtime packages.
// Named zones remain available when the host and GOROOT have no zoneinfo files.
// The standard library still prefers an available external timezone database.
package tzdata

import _ "time/tzdata"
