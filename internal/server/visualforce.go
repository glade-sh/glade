package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func (s *Server) handleVisualforcePage(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		writeSalesforceError(w, errUnknownEndpoint, "missing Visualforce page name")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleVisualforcePageGet(w, r, parts)
	case http.MethodPost:
		s.handleVisualforcePagePost(w, r, parts)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleVisualforcePageGet(w http.ResponseWriter, r *http.Request, parts []string) {
	_ = r
	s.renderVisualforceResponse(w, parts, nil, "", nil)
}

func (s *Server) handleVisualforcePagePost(w http.ResponseWriter, r *http.Request, parts []string) {
	if err := r.ParseForm(); err != nil {
		writeSalesforceError(w, errMalformedJSON, "failed to parse Visualforce form: "+err.Error())
		return
	}
	formValues := make(map[string]string, len(r.PostForm))
	for key, values := range r.PostForm {
		if len(values) > 0 {
			formValues[key] = values[len(values)-1]
		}
	}
	encoded := formValues[visualforce.ViewStateFormFieldName()]
	var payload *visualforce.ViewStatePayload
	if strings.TrimSpace(encoded) != "" {
		decoded, err := visualforce.DecodeViewState(encoded, nil)
		if err != nil {
			if errors.Is(err, visualforce.ErrViewStateTampered) || errors.Is(err, visualforce.ErrViewStateInvalid) || errors.Is(err, visualforce.ErrViewStateExpired) {
				writeSalesforceError(w, errUnsupportedFeature, err.Error())
				return
			}
			writeSalesforceError(w, errUnsupportedFeature, "failed to decode view state: "+err.Error())
			return
		}
		if err := visualforce.VerifyViewStateCSRF(decoded, formValues["__vf_csrf"]); err != nil {
			writeSalesforceError(w, errUnsupportedFeature, err.Error())
			return
		}
		payload = &decoded
	}
	action := strings.TrimSpace(formValues[visualforce.ViewStateActionFieldName()])
	s.renderVisualforceResponse(w, parts, payload, action, formValues)
}

func (s *Server) renderVisualforceResponse(w http.ResponseWriter, parts []string, viewState *visualforce.ViewStatePayload, action string, formValues map[string]string) {
	w.Header().Del("Content-Type")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	pageName := strings.TrimSpace(strings.Join(parts, "/"))
	pageFile, ok, err := lookupPageForRender(s.Source.Project, pageName)
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown Visualforce page")
		return
	}
	_ = pageFile

	machine, setupErr := s.visualforceRuntime()
	if setupErr != nil {
		writeSalesforceError(w, errUnsupportedFeature, setupErr.Error())
		return
	}
	vfIndex, err := visualforce.LoadProject(s.Source.Project)
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	visualforce.SetVMRenderEnvironment(machine, s.Source.Project)

	req := visualforce.PageRenderRequest{
		Project:    s.Source.Project,
		VFIndex:    vfIndex,
		Org:        s.Org,
		Machine:    machine,
		PageName:   pageName,
		PageURL:    "/apex/" + pageName,
		ViewState:  viewState,
		FormValues: formValues,
		Action:     action,
		Debug:      true,
	}
	if cfg, ok := s.lightningBootstrapConfigLocked(); ok {
		req.LightningBootstrap = cfg
	}
	result, err := visualforce.RenderPage(req)
	if err != nil && result.HTML == "" {
		if result.Error != nil {
			writeSalesforceError(w, errUnsupportedFeature, result.Error.Error())
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	if result.HTML == "" {
		writeSalesforceError(w, errUnsupportedFeature, "empty Visualforce render result")
		return
	}
	fmt.Fprint(w, result.HTML)
}

func (s *Server) visualforceRuntime() (*vm.VM, error) {
	if s.runtimeErr != nil {
		return nil, fmt.Errorf("Visualforce runtime setup failed: %w", s.runtimeErr)
	}
	machine := vm.New(nil)
	if s.runtime != nil {
		machine = s.runtime.CloneRuntime(nil)
	}
	if s.Org == nil {
		empty := storage.NewOrgState()
		s.Org = &empty
	}
	machine.SetOrg(s.Org)
	namespace := ""
	if s.Source.Project.Namespace != "" {
		namespace = s.Source.Project.Namespace
	} else if s.Index != nil {
		namespace = s.Index.Project.Namespace
	} else if s.Org != nil {
		namespace = s.Org.Namespace
	}
	if strings.TrimSpace(namespace) != "" {
		machine.SetCurrentNamespace(namespace)
	}
	if s.Index != nil && s.runtime == nil {
		if err := apextest.RegisterProjectRuntimeForRequest(machine, *s.Index); err != nil {
			return nil, fmt.Errorf("Visualforce runtime setup failed: %w", err)
		}
	}
	return machine, nil
}

func lookupPageForRender(p project.Project, name string) (string, bool, error) {
	if name == "" {
		return "", false, nil
	}
	idx, err := visualforce.LoadProject(p)
	if err != nil {
		return "", false, err
	}
	if page, ok := idx.Page(name); ok {
		return page.File, true, nil
	}
	return "", false, nil
}
