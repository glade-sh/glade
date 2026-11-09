package apextest

import (
	_ "embed"
	"strings"
	"sync"
)

// Only fields present in the native describe inventory have an observed
// deployed default. Keep uncaptured fields on their prior path.
//
//go:embed testdata/conformance/deployed_permissions_defaults/fields-62.0.txt
var deployedPermissionFields62 string

//go:embed testdata/conformance/deployed_permissions_defaults/fields-67.0.txt
var deployedPermissionFields67 string

//go:embed testdata/conformance/deployed_permissions_defaults/enterprise/fields-62.0.txt
var enterpriseDeployedPermissionFields62 string

//go:embed testdata/conformance/deployed_permissions_defaults/enterprise/fields-67.0.txt
var enterpriseDeployedPermissionFields67 string

//go:embed testdata/conformance/deployed_permissions_defaults/features/fields-62.0.txt
var featureDeployedPermissionFields62 string

//go:embed testdata/conformance/deployed_permissions_defaults/features/fields-67.0.txt
var featureDeployedPermissionFields67 string

//go:embed testdata/conformance/deployed_permissions_defaults/order_sales/fields-62.0.txt
var orderSalesDeployedPermissionFields62 string

//go:embed testdata/conformance/deployed_permissions_defaults/order_sales/fields-67.0.txt
var orderSalesDeployedPermissionFields67 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-62.0.txt
var versionDeployedPermissionFields62 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-63.0.txt
var versionDeployedPermissionFields63 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-64.0.txt
var versionDeployedPermissionFields64 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-65.0.txt
var versionDeployedPermissionFields65 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-66.0.txt
var versionDeployedPermissionFields66 string

//go:embed testdata/conformance/deployed_permissions_defaults/versions/fields-67.0.txt
var versionDeployedPermissionFields67 string

var deployedPermissionFieldCache struct {
	once   sync.Once
	fields map[string]uint8
}

// deployedPermissionFields parses the embedded inventories on first use rather
// than at package init, which every process paid.
func deployedPermissionFields() map[string]uint8 {
	deployedPermissionFieldCache.once.Do(func() {
		deployedPermissionFieldCache.fields = loadDeployedPermissionFields()
	})
	return deployedPermissionFieldCache.fields
}

func loadDeployedPermissionFields() map[string]uint8 {
	fields := make(map[string]uint8)
	inventories := []struct {
		contents string
		mask     uint8
	}{
		{deployedPermissionFields62, 1}, {enterpriseDeployedPermissionFields62, 1},
		{featureDeployedPermissionFields62, 1}, {orderSalesDeployedPermissionFields62, 1},
		{versionDeployedPermissionFields62, 1}, {versionDeployedPermissionFields63, 2},
		{versionDeployedPermissionFields64, 4}, {versionDeployedPermissionFields65, 8},
		{versionDeployedPermissionFields66, 16}, {versionDeployedPermissionFields67, 32},
		{deployedPermissionFields67, 32}, {enterpriseDeployedPermissionFields67, 32},
		{featureDeployedPermissionFields67, 32}, {orderSalesDeployedPermissionFields67, 32},
	}
	for _, inventory := range inventories {
		for _, name := range strings.Fields(inventory.contents) {
			fields[strings.ToLower(name)] |= inventory.mask
		}
	}
	return fields
}

func observedDeployedPermissionField(api, field string) bool {
	mask := deployedPermissionFields()[strings.ToLower(field)]
	switch api {
	case "62.0":
		return mask&1 != 0
	case "63.0":
		return mask&2 != 0
	case "64.0":
		return mask&4 != 0
	case "65.0":
		return mask&8 != 0
	case "66.0":
		return mask&16 != 0
	case "67.0":
		return mask&32 != 0
	default:
		return mask&33 == 33
	}
}
