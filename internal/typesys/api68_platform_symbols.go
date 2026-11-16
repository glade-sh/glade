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
	// crypto-ec and crypto-jwe deploy at 67.0 and 68.0 (CRYPTO-DEPLOY-*,
	// JWE-DEPLOY-*): the EC key pair helpers, parseJwe and the 9-argument
	// JWE encrypt and decrypt overloads compile.
	{Name: "Crypto", Methods: []StandardMethodSpec{
		{Name: "generateKeyPair", ReturnType: "Blob", Parameters: []string{"String"}, Static: true},
		{Name: "getPrivateKey", ReturnType: "Blob", Parameters: []string{"Blob"}, Static: true},
		{Name: "getPublicKey", ReturnType: "Blob", Parameters: []string{"Blob"}, Static: true},
		{Name: "parseJwe", ReturnType: "System.JweToken", Parameters: []string{"Blob"}, Static: true},
		{Name: "encrypt", ReturnType: "Blob", Parameters: []string{"String", "String", "Blob", "Blob", "Blob", "Blob", "Blob", "Blob", "Blob"}, Static: true},
		{Name: "decrypt", ReturnType: "Blob", Parameters: []string{"String", "String", "Blob", "Blob", "Blob", "Blob", "Blob", "Blob", "Blob"}, Static: true},
	}},
	// crypto-jwe JWE-CONTAINER-67/68: the 8-argument container and getters.
	{
		Name:                "JweToken",
		SuperClass:          "Object",
		Constructors:        [][]string{{"String", "String", "Blob", "Blob", "Blob", "Blob", "Blob", "Blob"}},
		ReplaceConstructors: true,
		Methods: []StandardMethodSpec{
			{Name: "getAlgorithm", ReturnType: "String"},
			{Name: "getEncryptionAlgorithm", ReturnType: "String"},
			{Name: "getEphemeralPublicKey", ReturnType: "Blob"},
			{Name: "getEncryptedKey", ReturnType: "Blob"},
			{Name: "getIv", ReturnType: "Blob"},
			{Name: "getCiphertext", ReturnType: "Blob"},
			{Name: "getTag", ReturnType: "Blob"},
			{Name: "getAad", ReturnType: "Blob"},
			{Name: "toString", ReturnType: "String"},
		},
	},
}
