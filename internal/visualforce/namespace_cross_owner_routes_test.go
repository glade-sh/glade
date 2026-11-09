package visualforce

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A Visualforce controller crosses the same project semantic gate as Apex.
func TestCrossNamespaceOwnerVFController(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := writeControllerPageProject(t, `public class ProbeController { public String value { get; set; } public void run() { host.ForeignShape bad; } }`)
			child := filepath.Join(p.Root, "child")
			foreign := filepath.Join(child, "ForeignShape.cls")
			writeFile(t, foreign, "public class ForeignShape {}")
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			writeFile(t, foreign+"-meta.xml", meta)
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), meta)
			p.Namespace = "host"
			p.SourceAPIVersion = api
			p.SourceNamespaces = []string{"other"}
			p.SourceNamespaceRoots = map[string]string{child: "other"}
			p.ApexFiles = append(p.ApexFiles, foreign)
			compilation := gateLikeCompilation(t, p)
			if names := PageNamesWithControllerCompilation(p, compilation); !onlyPlain(names) {
				t.Fatalf("controller with cross-namespace type was accepted: %v", names)
			}
		})
	}
}

func TestCrossNamespaceOwnerVFField(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := writeControllerPageProject(t, `public class ProbeController { public host.ForeignShape bad; public String value { get; set; } }`)
			child := filepath.Join(p.Root, "child")
			foreign := filepath.Join(child, "ForeignShape.cls")
			writeFile(t, foreign, "public class ForeignShape {}")
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			writeFile(t, foreign+"-meta.xml", meta)
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), meta)
			p.Namespace = "host"
			p.SourceAPIVersion = api
			p.SourceNamespaces = []string{"other"}
			p.SourceNamespaceRoots = map[string]string{child: "other"}
			p.ApexFiles = append(p.ApexFiles, foreign)
			compilation := gateLikeCompilation(t, p)
			for attempt := 0; attempt < 2; attempt++ {
				if names := PageNamesWithControllerCompilation(p, compilation); !onlyPlain(names) {
					t.Fatalf("attempt %d: controller with cross-namespace field type was accepted: %v", attempt, names)
				}
			}
		})
	}
}

func TestCrossNamespaceOwnerVFParameter(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := writeControllerPageProject(t, `public class ProbeController { public String value { get; set; } public void accept(host.ForeignShape bad) {} }`)
			child := filepath.Join(p.Root, "child")
			foreign := filepath.Join(child, "ForeignShape.cls")
			writeFile(t, foreign, "public class ForeignShape {}")
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			writeFile(t, foreign+"-meta.xml", meta)
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), meta)
			p.Namespace = "host"
			p.SourceAPIVersion = api
			p.SourceNamespaces = []string{"other"}
			p.SourceNamespaceRoots = map[string]string{child: "other"}
			p.ApexFiles = append(p.ApexFiles, foreign)
			compilation := gateLikeCompilation(t, p)
			for attempt := 0; attempt < 2; attempt++ {
				if names := PageNamesWithControllerCompilation(p, compilation); !onlyPlain(names) {
					t.Fatalf("attempt %d: controller with cross-namespace parameter type was accepted: %v", attempt, names)
				}
			}
		})
	}
}

func TestCrossNamespaceSameNameVFController(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := writeControllerPageProject(t, `public class ProbeController { public String value { get; set; } public void run() { host.SharedShape first; other.SharedShape second; } }`)
			local := filepath.Join(p.Root, "force-app/main/default/classes/SharedShape.cls")
			child := filepath.Join(p.Root, "child")
			foreign := filepath.Join(child, "SharedShape.cls")
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			for _, file := range []string{local, foreign} {
				writeFile(t, file, "public class SharedShape {}")
				writeFile(t, file+"-meta.xml", meta)
			}
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), meta)
			p.Namespace = "host"
			p.SourceAPIVersion = api
			p.SourceNamespaces = []string{"other"}
			p.SourceNamespaceRoots = map[string]string{child: "other"}
			p.ApexFiles = append(p.ApexFiles, local, foreign)
			compilation := gateLikeCompilation(t, p)
			for attempt := 0; attempt < 2; attempt++ {
				if names := PageNamesWithControllerCompilation(p, compilation); len(names) != 2 {
					t.Fatalf("attempt %d: accepted controller pages=%v", attempt, names)
				}
			}
		})
	}
}

func TestCrossNamespaceVFPageFloor(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprintf("same-name=%v", same), func(t *testing.T) {
			body := `public class ProbeController { public String value { get; set; } public void accept(host.ForeignShape bad) {} }`
			if same {
				body = `public class ProbeController { public String value { get; set; } public void run() { host.SharedShape first; other.SharedShape second; } }`
			}
			p := writeControllerPageProject(t, body)
			child := filepath.Join(p.Root, "child")
			meta := `<ApexClass><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`
			foreign := filepath.Join(child, "ForeignShape.cls")
			name := "ForeignShape"
			if same {
				foreign = filepath.Join(child, "SharedShape.cls")
				name = "SharedShape"
				local := filepath.Join(p.Root, "force-app/main/default/classes/SharedShape.cls")
				writeFile(t, local, "public class SharedShape {}")
				writeFile(t, local+"-meta.xml", meta)
				p.ApexFiles = append(p.ApexFiles, local)
			}
			writeFile(t, foreign, "public class "+name+" {}")
			writeFile(t, foreign+"-meta.xml", meta)
			writeFile(t, filepath.Join(p.Root, "force-app/main/default/pages/Probe.page-meta.xml"), `<ApexPage><apiVersion>59.0</apiVersion><label>Probe</label></ApexPage>`)
			p.Namespace = "host"
			p.SourceAPIVersion = "62.0"
			p.SourceNamespaces = []string{"other"}
			p.SourceNamespaceRoots = map[string]string{child: "other"}
			p.ApexFiles = append(p.ApexFiles, foreign)
			names := PageNamesWithControllerCompilation(p, gateLikeCompilation(t, p))
			if same && len(names) != 2 {
				t.Fatalf("same-name controller pages=%v", names)
			}
			if !same && !onlyPlain(names) {
				t.Fatalf("invalid controller pages=%v", names)
			}
		})
	}
}

func TestCrossNamespaceOwnerVFCallerPositions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, body := range map[string]string{"class-literal": "Type local = host.ForeignShape.class;", "catch": "try { Integer local = 1; } catch (host.ForeignShape e) { }", "foreach": "for (host.ForeignShape local : new List<host.ForeignShape>()) { }"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				p := writeControllerPageProject(t, "public class ProbeController { public String value { get; set; } public void run() { "+body+" } }")
				child := filepath.Join(p.Root, "child")
				foreign := filepath.Join(child, "ForeignShape.cls")
				meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
				writeFile(t, foreign, "public class ForeignShape {}")
				writeFile(t, foreign+"-meta.xml", meta)
				writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), meta)
				p.Namespace = "host"
				p.SourceAPIVersion = api
				p.SourceNamespaces = []string{"other"}
				p.SourceNamespaceRoots = map[string]string{child: "other"}
				p.ApexFiles = append(p.ApexFiles, foreign)
				if names := PageNamesWithControllerCompilation(p, gateLikeCompilation(t, p)); !onlyPlain(names) {
					t.Fatalf("controller with %s accepted: %v", name, names)
				}
			})
		}
	}
}
