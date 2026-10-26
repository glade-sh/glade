package storage

import "github.com/glade-sh/glade/internal/project"

// EmailServiceRejections lists the email-service files of p that a local run
// skips, with the native deploy-time text. It seeds a scratch org, so it
// reports the same file, class, user and local-part failures as ApplyProject.
func EmailServiceRejections(p project.Project) []project.EmailServiceRejection {
	services, rejections := project.LoadEmailServicesTolerant(p, nil)
	if len(services) == 0 {
		return rejections
	}
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	for _, service := range services {
		if err := ApplyEmailServices(&org, []project.EmailService{service}); err != nil {
			rejections = append(rejections, project.EmailServiceRejection{File: service.SourceFile, Message: err.Error()})
		}
	}
	return rejections
}
