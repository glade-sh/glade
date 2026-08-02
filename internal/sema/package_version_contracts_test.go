package sema

import (
	"testing"
)

func TestPackageVersionClassShapesMatchAPI67(t *testing.T) {
	accepted := map[string]string{
		"type": `public class CB102_Type {
    public static void run() {
        Package.Version value = (Package.Version)null;
        Object sink = value;
    }
}`,
		"method": `public class CB102_Method {
    public static void run() {
        Object sink = ((Package.Version)null).toString();
    }
}`,
		"members": `public class CB102_Members {
    public static void run() {
        Object equalsSink = ((Package.Version)null).equals((Object)null);
        Object hashCodeSink = ((Package.Version)null).hashCode();
        Object greaterThanSink = ((Package.Version)null).isGreaterThan((Package.Version)null);
        Object greaterThanOrEqualSink = ((Package.Version)null).isGreaterThanOrEqual((Package.Version)null);
        Object lessThanSink = ((Package.Version)null).isLessThan((Package.Version)null);
        Object lessThanOrEqualSink = ((Package.Version)null).isLessThanOrEqual((Package.Version)null);
        Object toStringSink = ((Package.Version)null).toString();
    }
}`,
		"runAs": `public class CB102_RunAs {
    public static void run() {
        Package.Version value = null;
        if (value != null) {
            System.runAs(value) {
                System.debug(value);
            }
        }
    }
}`,
	}
	for name, source := range accepted {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{name + ".cls": source}, "67.0")
			if result.HasErrors() {
				t.Fatalf("API 67 rejected Package.Version %s shape: %#v", name, result.Diagnostics)
			}
		})
	}

	rejected := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"CB102_Constructor.cls": `public class CB102_Constructor {
    public static void run() {
        Object sink = new Package.Version();
    }
}`,
	}, "67.0")
	if !rejected.HasErrors() {
		t.Fatal("API 67 accepted rejected Package.Version zero-argument constructor")
	}
}
