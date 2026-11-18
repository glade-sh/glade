package project

import (
	"path/filepath"
	"testing"
)

// writeSiblingFixture lays out a consumer that declares package Core and, when
// withCore is true, a Core project in a directory beside it.
func writeSiblingFixture(t *testing.T, withCore bool, consumerConfig string) (consumerRoot, coreRoot string) {
	t.Helper()
	root := t.TempDir()
	coreRoot = filepath.Join(root, "libs", "core")
	consumerRoot = filepath.Join(root, "libs", "consumer")
	if withCore {
		writeFile(t, filepath.Join(coreRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Core"}]
}`)
		writeFile(t, filepath.Join(coreRoot, "force-app/main/default/classes/CoreHelper.cls"), "global class CoreHelper {}")
	}
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Consumer","dependencies":[{"package":"Core"}]}]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	if consumerConfig != "" {
		writeFile(t, filepath.Join(consumerRoot, "glade.yml"), consumerConfig)
	}
	return consumerRoot, coreRoot
}

func dependencyShape(p Project) (loaded int, codes []string) {
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" {
			loaded++
		}
	}
	for _, d := range p.DependencyDiagnostics {
		codes = append(codes, d.Code+":"+d.Namespace)
	}
	return loaded, codes
}

// By default a project resolves the same whether or not a matching sibling
// project sits beside it.
func TestSiblingDependencyScanOffGivesIdenticalResults(t *testing.T) {
	t.Setenv(SiblingDependencyScanEnvVar, "")
	standalone, _ := writeSiblingFixture(t, false, "")
	withSibling, _ := writeSiblingFixture(t, true, "")
	a, err := Load(standalone)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(withSibling)
	if err != nil {
		t.Fatal(err)
	}
	aLoaded, aCodes := dependencyShape(a)
	bLoaded, bCodes := dependencyShape(b)
	if aLoaded != 0 || bLoaded != 0 {
		t.Fatalf("loaded dependencies standalone=%d sibling=%d, want 0 and 0", aLoaded, bLoaded)
	}
	if len(aCodes) != 1 || len(bCodes) != 1 || aCodes[0] != bCodes[0] || aCodes[0] != "dependency_missing:Core" {
		t.Fatalf("diagnostics standalone=%v sibling=%v, want the same dependency_missing", aCodes, bCodes)
	}
	if len(a.ApexFiles) != len(b.ApexFiles) {
		t.Fatalf("apex files standalone=%d sibling=%d", len(a.ApexFiles), len(b.ApexFiles))
	}
}

func TestSiblingDependencyScanOptInByEnvironment(t *testing.T) {
	t.Setenv(SiblingDependencyScanEnvVar, "1")
	consumer, _ := writeSiblingFixture(t, true, "")
	p, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if loaded, codes := dependencyShape(p); loaded != 1 || len(codes) != 0 {
		t.Fatalf("loaded=%d diagnostics=%v, want sibling dependency loaded", loaded, codes)
	}
}

func TestSiblingDependencyScanOptInByConfig(t *testing.T) {
	t.Setenv(SiblingDependencyScanEnvVar, "")
	consumer, _ := writeSiblingFixture(t, true, "project:\n  siblingDependencyScan: true\n")
	p, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if loaded, codes := dependencyShape(p); loaded != 1 || len(codes) != 0 {
		t.Fatalf("loaded=%d diagnostics=%v, want sibling dependency loaded", loaded, codes)
	}
}

func TestSiblingDependencyScanConfigFalseStaysOff(t *testing.T) {
	t.Setenv(SiblingDependencyScanEnvVar, "")
	consumer, _ := writeSiblingFixture(t, true, "project:\n  siblingDependencyScan: false\n")
	p, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if loaded, _ := dependencyShape(p); loaded != 0 {
		t.Fatalf("loaded=%d, want 0 with the scan off", loaded)
	}
}

// A declared dependencyPaths entry resolves with the scan off, and gives the
// same result whether or not other siblings exist.
func TestDeclaredDependencyPathsResolveWithScanOff(t *testing.T) {
	t.Setenv(SiblingDependencyScanEnvVar, "")
	consumer, coreRoot := writeSiblingFixture(t, true, "")
	writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: ["+`"`+filepath.ToSlash(filepath.Dir(coreRoot))+`"`+"]\n")
	p, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if loaded, codes := dependencyShape(p); loaded != 1 || len(codes) != 0 {
		t.Fatalf("loaded=%d diagnostics=%v, want declared dependency loaded", loaded, codes)
	}
}
