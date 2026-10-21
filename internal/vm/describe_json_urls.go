package vm

import "github.com/glade-sh/glade/internal/apexversion"

// DJ01..DJ04 in describe_json_version_native.json observe these relative
// platform-68 URLs for deferred User/Account describes, independently of the
// source and transport APIs. Other objects, options and namespaces are unprobed.
// Project the JSON property without changing cached metadata or record URLs.
func (vm *VM) describeJSONURLs(value Value) (map[string]string, bool) {
	if vm == nil || value.Kind != ValueObject || value.classInstance || value.Type != "Schema.DescribeSObjectResult" ||
		vm.currentCallerNamespace() != "" || (vm.Org != nil && vm.Org.Namespace != "") ||
		value.Fields["sObjectDescribeOption"].Text != "DEFERRED" {
		return nil, false
	}
	if _, _, exists := objectFieldValue(value, "urls"); exists {
		return nil, false
	}
	api, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok || (api != 62 && api != 67 && api != 68) {
		return nil, false
	}
	name := value.Fields["name"]
	if name.Kind != ValueString || (name.Text != "User" && name.Text != "Account") {
		return nil, false
	}
	base := "/services/data/v68.0/sobjects/" + name.Text
	urls := map[string]string{
		"sobject":        base,
		"describe":       base + "/describe",
		"layouts":        base + "/describe/layouts",
		"compactLayouts": base + "/describe/compactLayouts",
		"quickActions":   base + "/quickActions",
		"rowTemplate":    base + "/{ID}",
	}
	if name.Text == "User" {
		urls["passwordUtilities"] = base + "/{ID}/password"
		urls["namedLayouts"] = base + "/describe/namedLayouts/{LayoutName}"
	} else {
		urls["listviews"] = base + "/listviews"
		urls["approvalLayouts"] = base + "/describe/approvalLayouts"
	}
	return urls, true
}
