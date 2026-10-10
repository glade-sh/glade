package apextest

import (
	"path/filepath"
	"testing"
)

func TestBitwiseComplementAPI52(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeComplement52Proof.cls"), "@IsTest private class GladeComplement52Proof {\n    private enum Feature { FIRST, SECOND, THIRD }\n    @IsTest static void clearSelectedBitPreservesOthers() {\n        Integer mask = 0;\n        mask |= 1 << Feature.FIRST.ordinal();\n        mask |= 1 << Feature.SECOND.ordinal();\n        mask |= 1 << Feature.THIRD.ordinal();\n        System.assertEquals(7, mask);\n        mask &= ~(1 << Feature.SECOND.ordinal());\n        System.assertEquals(5, mask);\n        System.assertEquals(0, mask & (1 << Feature.SECOND.ordinal()));\n        mask &= ~(1 << Feature.SECOND.ordinal());\n        System.assertEquals(5, mask);\n        mask ^= 1 << Feature.THIRD.ordinal();\n        System.assertEquals(1, mask);\n    }\n    @IsTest static void complementRetainsSignedBits() {\n        System.assertEquals(-1, ~0);\n        System.assertEquals(0, ~(-1));\n        System.assertEquals(-6, ~5);\n        Integer input = 1025;\n        System.assertEquals(input, ~(~input));\n        System.assertEquals(0, input & ~input);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeComplement52Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>52.0</apiVersion><status>Active</status></ApexClass>")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"52.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if s := run.Summary(); s.Total != 2 || s.Passed != 2 || s.Errors != 0 {
		t.Fatalf("complement proof: %#v", run)
	}
}
