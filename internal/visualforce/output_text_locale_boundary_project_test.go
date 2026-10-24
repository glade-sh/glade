package visualforce_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestOutputTextDateStyleLocaleBoundaryProjectRoutes(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			base := "force-app/main/default/"
			write(base+"classes/LocaleDateController.cls", `public class LocaleDateController {
 public Datetime getStamp() { return Datetime.newInstanceGmt(2024, 2, 9, 17, 6, 3); }
}`)
			write(base+"classes/LocaleDateController.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			patterns := []struct{ name, pattern, want string }{
				{"Short", "short", "2/9/24"},
				{"Numeric", "yyyy-MM-dd", "2024-02-09"},
			}
			bodies := map[string]string{}
			for _, pattern := range patterns {
				body := `<apex:page controller="LocaleDateController" showHeader="false" sidebar="false" standardStylesheets="false"><div data-case="date"><apex:outputText value="{0,date,` + pattern.pattern + `}"><apex:param value="{!stamp}"/></apex:outputText></div></apex:page>`
				bodies[pattern.name] = body
				write(base+"pages/LocaleDate"+pattern.name+".page", body)
				write(base+"pages/LocaleDate"+pattern.name+".page-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Locale date</label></ApexPage>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index, err := visualforce.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(typesys.Build(p, sch), nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, locale := range []string{"en_US", "de_DE", "fr_FR"} {
				for _, pattern := range patterns {
					for _, route := range []string{"Markup", "RenderPage", "RenderPageURL", "RenderPageForTest", "AnonymousGetContent", "TestContextGetContent"} {
						t.Run(locale+"/"+pattern.name+"/"+route, func(t *testing.T) {
							machine := vm.New(nil)
							if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
								t.Fatal(err)
							}
							user := storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale)}}
							machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{"User": {Records: map[storage.ID]storage.Record{user.ID: user}}}})
							machine.SetCurrentUser(user)
							visualforce.SetVMRenderEnvironment(machine, p)
							defer visualforce.ClearVMRenderEnvironment(machine)
							page := "LocaleDate" + pattern.name
							var rendered string
							var renderErr error
							switch route {
							case "Markup":
								tree, err := visualforce.ParseMarkupTree(bodies[pattern.name])
								if err != nil {
									t.Fatal(err)
								}
								stamp := vm.Object("Datetime")
								stamp.Fields["value"] = vm.String("2024-02-09T17:06:03Z")
								controller := vm.Object("LocaleDateController")
								controller.Fields["stamp"] = stamp
								rendered, renderErr = visualforce.RenderMarkupTree(tree, &visualforce.RenderContext{VM: machine, PageMeta: visualforce.Page{APIVersion: api}, Expression: &visualforce.ExpressionContext{Controller: controller}})
							case "RenderPage":
								result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: page})
								rendered, renderErr = result.HTML, err
								if renderErr == nil && result.Error != nil {
									renderErr = result.Error
								}
							case "RenderPageURL":
								result, err := visualforce.RenderPageURL(machine, "/apex/"+page, false)
								rendered, renderErr = result.String(), err
							case "RenderPageForTest":
								rendered, renderErr = visualforce.RenderPageForTest(machine, root, page)
							default:
								if route == "TestContextGetContent" {
									machine.EnableTestContext()
								}
								program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", page))
								if err != nil {
									t.Fatal(err)
								}
								result, err := machine.Execute(program)
								renderErr = err
								if err == nil {
									if len(result.Debug) != 1 {
										t.Fatalf("debug rows=%d, want 1", len(result.Debug))
									}
									rendered = result.Debug[0]
								}
							}
							// The existing Visualforce locale resolver rejects active test-user
							// contexts and falls back to en_US on that local route.
							if locale != "en_US" && pattern.name == "Short" && route != "TestContextGetContent" {
								var runtimeErr *vm.RuntimeError
								if !errors.As(renderErr, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
									t.Fatalf("date=%q error=%v, want UnsupportedFeature locale boundary", outputTextCaseHTML(t, rendered)["date"], renderErr)
								}
							} else {
								if renderErr != nil {
									t.Fatal(renderErr)
								}
								if got := strings.TrimSpace(outputTextCaseHTML(t, rendered)["date"]); got != pattern.want {
									t.Fatalf("date=%q, want %q", got, pattern.want)
								}
							}
						})
					}
				}
			}
		})
	}
}
