package vm

import "testing"

func TestAccessLevelClassifierSourceEnumAndPlatformConstructorDirectVMNative(t *testing.T) {
	// source_enum_access_json_routes/org.tsv:2,4,6,8: a source enum is serializable.
	source, ok := namedEnumStaticValue("AccessLevel", []string{"LOCAL", "OTHER"}, "AccessLevel.LOCAL")
	if !ok || isAccessLevelValue(source) {
		t.Fatal("source enum was classified as a platform AccessLevel")
	}
	// access_level_json_routes/org.tsv:2-17: platform constants/clones retain identity and reject JSON.
	for _, mode := range []string{"USER_MODE", "SYSTEM_MODE"} {
		original := accessLevelValue(mode)
		for route, value := range map[string]Value{"constant": original, "clone": accessLevelClone(original), "detached": cloneValueDetachedPreserveRefs(original)} {
			t.Run(mode+"/"+route, func(t *testing.T) {
				if !isAccessLevelValue(value) || !accessLevelUsesReferenceEquality(original, value) {
					t.Fatal("platform AccessLevel lost its comparison route")
				}
			})
		}
	}
}
