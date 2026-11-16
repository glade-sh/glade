package typesys

// api68PlatformSymbolOverlays declares Winter '27 platform members that the
// API 68 native capture compiled and ran (apextest
// testdata/conformance/api68_native). Parameter names are positional: the
// capture proves arity and argument types, not the declared names.
var api68PlatformSymbolOverlays = []StandardSymbolSpec{
	// limits/LIMITS-DC-* and LIMITS-FLEX-* at 67.0 and 68.0.
	{Name: "Limits", Methods: []StandardMethodSpec{
		{Name: "getDataCloudCursors", ReturnType: "Integer", Static: true},
		{Name: "getLimitDataCloudCursors", ReturnType: "Integer", Static: true},
		{Name: "getDataCloudCursorRows", ReturnType: "Integer", Static: true},
		{Name: "getLimitDataCloudCursorRows", ReturnType: "Integer", Static: true},
		{Name: "getDataCloudFlexCredits", ReturnType: "Integer", Static: true},
		{Name: "getLimitDataCloudFlexCredits", ReturnType: "Integer", Static: true},
	}},
}
