package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// HTTP responses consume debug text, return values and exception stacks, not
// VM statement events. Exercise both trace settings on the same request template.
func TestServerRequestTraceModesPreserveHTTPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := newRequestTraceServer(t, api)
			responses := make(map[string]string)
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("trace_%t", enabled), func(t *testing.T) {
					srv.runtime.SetTraceEnabled(enabled)
					for _, route := range []struct {
						name, method, path, body, want string
						status                         int
					}{
						{"apex_rest", "GET", "/services/apexrest/request-trace", "", `"ready"`, 200},
						{"apex_rest_exception", "GET", "/services/apexrest/request-trace?fail=true", "", "trace-route-failure", 501},
						{"lightning_imperative", "POST", "/lightning/apex/RequestTraceController/echo", `{"fail":false}`, `"data":"ready"`, 200},
						{"lightning_imperative_exception", "POST", "/lightning/apex/RequestTraceController/echo", `{"fail":true}`, "trace-route-failure", 200},
						{"lightning_wire", "POST", "/lightning/wire/apex", `{"className":"RequestTraceController","method":"echo","params":{"fail":false},"cacheable":true}`, `"data":"ready"`, 200},
						{"lightning_wire_exception", "POST", "/lightning/wire/apex", `{"className":"RequestTraceController","method":"echo","params":{"fail":true},"cacheable":true}`, "trace-route-failure", 200},
						{"coverage_boundary", "GET", "/services/data/v67.0/tooling/coverage", "", "ApexCodeCoverage resources are not implemented", 501},
					} {
						t.Run(route.name, func(t *testing.T) {
							req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
							req.Header.Set("Content-Type", "application/json")
							rec := httptest.NewRecorder()
							srv.ServeHTTP(rec, req)
							if rec.Code != route.status || !strings.Contains(rec.Body.String(), route.want) {
								t.Fatalf("status=%d body=%s; want status=%d containing %s", rec.Code, rec.Body.String(), route.status, route.want)
							}
							if strings.Contains(route.name, "exception") && !strings.Contains(rec.Body.String(), "RequestTrace") {
								t.Fatalf("exception lost class context: %s", rec.Body.String())
							}
							if !enabled {
								responses[route.name] = rec.Body.String()
							} else if rec.Body.String() != responses[route.name] {
								t.Fatalf("response changed with trace enabled: off=%s on=%s", responses[route.name], rec.Body.String())
							}
						})
					}
					t.Run("visualforce_get_post_ajax", func(t *testing.T) {
						first := httptest.NewRecorder()
						srv.ServeHTTP(first, httptest.NewRequest("GET", "/apex/Remote", nil))
						if first.Code != 200 || !strings.Contains(first.Body.String(), "ready") {
							t.Fatalf("GET status=%d body=%s", first.Code, first.Body.String())
						}
						for _, ajax := range []bool{false, true} {
							form := url.Values{
								visualforce.ViewStateFormFieldName():   {extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName())},
								"__vf_csrf":                            {extractHTMLInput(first.Body.String(), "__vf_csrf")},
								visualforce.ViewStateActionFieldName(): {"{!save}"},
							}
							if ajax {
								form.Set("__vf_ajax", "1")
								form.Set("__vf_rerender", "f:message")
							}
							req := httptest.NewRequest("POST", "/apex/Remote", strings.NewReader(form.Encode()))
							req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
							rec := httptest.NewRecorder()
							srv.ServeHTTP(rec, req)
							if rec.Code != 200 || !strings.Contains(rec.Body.String(), "saved") {
								t.Fatalf("POST ajax=%t status=%d body=%s", ajax, rec.Code, rec.Body.String())
							}
						}
					})
					t.Run("visualforce_remoting", func(t *testing.T) {
						for _, fail := range []bool{false, true} {
							body := fmt.Sprintf(`[{"action":"RequestTraceController","method":"remoteEcho","data":[%t],"type":"rpc","tid":1}]`, fail)
							responses := postVisualforceRemotingWithViewState(t, srv, body)
							if len(responses) != 1 || responses[0].Status == fail {
								t.Fatalf("fail=%t responses=%#v", fail, responses)
							}
							if !fail && responses[0].Result != "ready" || fail && !strings.Contains(responses[0].Message, "trace-route-failure") {
								t.Fatalf("fail=%t response=%#v", fail, responses[0])
							}
						}
					})
				})
			}
		})
	}
}

func TestServerRequestTraceModesPreserveAnonymousLogs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := newRequestTraceServer(t, api)
			const source = "System.debug('before'); try { Integer n = 1 / 0; } catch (Exception e) { System.debug('caught'); }"
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("trace_%t", enabled), func(t *testing.T) {
					srv.runtime.SetTraceEnabled(enabled)
					// HTTP API versions have a separate supported window from
					// the project's source API version.
					path := "/services/data/v67.0/tooling/executeAnonymous"
					for _, method := range []string{"GET", "POST"} {
						t.Run("tooling_"+method, func(t *testing.T) {
							requestPath, body := path, ""
							if method == "GET" {
								requestPath += "?anonymousBody=" + url.QueryEscape(source)
							} else {
								data, err := json.Marshal(map[string]string{"anonymousBody": source})
								if err != nil {
									t.Fatal(err)
								}
								body = string(data)
							}
							req := httptest.NewRequest(method, requestPath, strings.NewReader(body))
							req.Header.Set("Content-Type", "application/json")
							rec := httptest.NewRecorder()
							srv.ServeHTTP(rec, req)
							var response struct {
								Success bool
								Logs    string
							}
							if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
								t.Fatal(err)
							}
							if rec.Code != 200 || !response.Success || response.Logs != "before\ncaught" {
								t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
							}
						})
					}
					t.Run("soap_anonymous", func(t *testing.T) {
						rec := httptest.NewRecorder()
						body := soapEnvelope("<apex:executeAnonymous><apex:String>" + source + "</apex:String></apex:executeAnonymous>")
						srv.ServeHTTP(rec, httptest.NewRequest("POST", "/services/Soap/s/67.0/00DLOCAL000000001", strings.NewReader(body)))
						result := decodeSOAPResult(t, rec.Body.String())
						if rec.Code != 200 || result["success"] != "true" {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
					})
				})
			}
		})
	}
}

func TestServerRequestTraceModesPreserveDebugHooks(t *testing.T) {
	srv := newRequestTraceServer(t, "67.0")
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("request_clone_trace_%t", enabled), func(t *testing.T) {
			machine := srv.runtime.CloneRuntime(nil)
			machine.SetTraceEnabled(enabled)
			var messages []string
			machine.SetDebugOutputSink(func(event vm.DebugEvent) { messages = append(messages, event.Message) })
			pauses := 0
			machine.SetDebugHooks(vm.DebugHooks{Step: true, OnPause: func(pause vm.DebugPause) vm.DebugAction {
				pauses++
				return vm.DebugActionContinue
			}})
			program, err := vm.CompileAnonymousWithOptions("RequestTraceController.echo(false); System.debug('debug-hook');", vm.CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"echo", "debug-hook"}
			if !reflect.DeepEqual(result.Debug, want) || !reflect.DeepEqual(messages, want) || pauses == 0 {
				t.Fatalf("debug=%v sink=%v pauses=%d", result.Debug, messages, pauses)
			}
			if enabled != (len(result.Trace) > 0) || enabled != (len(result.DebugEvents) > 0) {
				t.Fatalf("trace enabled=%t: events=%d debug events=%d", enabled, len(result.Trace), len(result.DebugEvents))
			}
		})
	}
}

func newRequestTraceServer(t *testing.T, api string) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	classes := map[string]string{
		"RequestTraceController": `public class RequestTraceController {
  public String message { get; set; }
  public RequestTraceController() { message = 'ready'; }
  public PageReference save() { System.debug('save'); message = 'saved'; return null; }
  @AuraEnabled(cacheable=true) public static String echo(Boolean fail) {
    System.debug('echo');
    if (fail) { throw new IllegalArgumentException('trace-route-failure'); }
    return 'ready';
  }
  @RemoteAction public static String remoteEcho(Boolean fail) { return echo(fail); }
}`,
		"RequestTraceResource": `@RestResource(urlMapping='/request-trace')
global class RequestTraceResource {
  @HttpGet global static String getIt() { return RequestTraceController.echo(RestContext.request.params.get('fail') == 'true'); }
}`,
	}
	for name, source := range classes {
		writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
		writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="RequestTraceController"><apex:form id="f"><apex:outputPanel id="message"><apex:outputText value="{!message}"/></apex:outputPanel><apex:commandButton value="Save" action="{!save}" reRender="message"/></apex:form></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = api
	srv := newVisualforceHTMLTestServer(t, &org, source)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	return srv
}
