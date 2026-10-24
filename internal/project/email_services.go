package project

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EmailService is the deployable metadata that backs email service setup rows.
type EmailService struct {
	HandlerDeclared             bool                  `xml:"-"`
	SourceFile                  string                `xml:"-"`
	XMLName                     xml.Name              `xml:"EmailServicesFunction"`
	ApexClass                   string                `xml:"apexClass"`
	FunctionName                string                `xml:"functionName"`
	AttachmentOption            string                `xml:"attachmentOption"`
	AuthenticationFailureAction string                `xml:"authenticationFailureAction"`
	AuthorizationFailureAction  string                `xml:"authorizationFailureAction"`
	FunctionInactiveAction      string                `xml:"functionInactiveAction"`
	OverLimitAction             string                `xml:"overLimitAction"`
	AuthorizedSenders           string                `xml:"authorizedSenders"`
	ErrorRoutingAddress         string                `xml:"errorRoutingAddress"`
	IsActive                    bool                  `xml:"isActive"`
	IsAuthenticationRequired    bool                  `xml:"isAuthenticationRequired"`
	IsErrorRoutingEnabled       bool                  `xml:"isErrorRoutingEnabled"`
	IsTextAttachmentsAsBinary   bool                  `xml:"isTextAttachmentsAsBinary"`
	IsTlsRequired               bool                  `xml:"isTlsRequired"`
	Addresses                   []EmailServiceAddress `xml:"emailServicesAddresses"`
}

type EmailServiceAddress struct {
	DeveloperName     string `xml:"developerName"`
	LocalPart         string `xml:"localPart"`
	RunAsUser         string `xml:"runAsUser"`
	AuthorizedSenders string `xml:"authorizedSenders"`
	IsActive          bool   `xml:"isActive"`
}

// EmailServiceMetadataFiles includes loaded package views and shims. Runtime
// snapshots use the same paths as the loader to retain one metadata generation.
func EmailServiceMetadataFiles(p Project) []string {
	files := map[string]bool{}
	seen := map[*Project]bool{}
	var visit func(*Project)
	visit = func(current *Project) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		for _, path := range current.EmailServiceFiles {
			files[path] = true
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project)
			}
		}
		for _, shim := range current.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project)
			}
		}
	}
	visit(&p)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// EmailServiceRejection names one email-service file that a local run skipped.
// Message keeps the native deploy-time text.
type EmailServiceRejection struct {
	File    string
	Message string
}

func LoadEmailServices(p Project, read func(string) ([]byte, error)) ([]EmailService, error) {
	services, rejections := loadEmailServices(p, read)
	if len(rejections) > 0 {
		return nil, fmt.Errorf("%s", rejections[0].Message)
	}
	return services, nil
}

// LoadEmailServicesTolerant returns every readable service and reports files it
// cannot read or parse, so one bad file does not hide the others.
func LoadEmailServicesTolerant(p Project, read func(string) ([]byte, error)) ([]EmailService, []EmailServiceRejection) {
	return loadEmailServices(p, read)
}

func loadEmailServices(p Project, read func(string) ([]byte, error)) ([]EmailService, []EmailServiceRejection) {
	if read == nil {
		read = os.ReadFile
	}
	var services []EmailService
	var rejections []EmailServiceRejection
	for _, path := range EmailServiceMetadataFiles(p) {
		data, err := read(path)
		if err != nil {
			rejections = append(rejections, EmailServiceRejection{File: path, Message: fmt.Sprintf("load email service %s: %v", path, err)})
			continue
		}
		var service EmailService
		if err := xml.Unmarshal(data, &service); err != nil {
			rejections = append(rejections, EmailServiceRejection{File: path, Message: fmt.Sprintf("load email service %s: %v", path, err)})
			continue
		}
		service.SourceFile = path
		var declaresHandler func(*Project) bool
		seen := map[*Project]bool{}
		declaresHandler = func(current *Project) bool {
			if current == nil || seen[current] {
				return false
			}
			seen[current] = true
			for _, path := range current.ApexFiles {
				if strings.EqualFold(filepath.Base(path), service.ApexClass+".cls") {
					return true
				}
			}
			for _, dep := range current.ManagedPackageDependencies {
				if dep.Status == "loaded" && declaresHandler(dep.Project) {
					return true
				}
			}
			for _, shim := range current.PackageShims {
				if shim.Status == "loaded" && declaresHandler(shim.Project) {
					return true
				}
			}
			return false
		}
		service.HandlerDeclared = declaresHandler(&p)
		services = append(services, service)
	}
	return services, rejections
}
