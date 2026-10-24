package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

// These are real project/HTTP/dev-command consumers, using an owned compiler
// stand-in that rejects entry before the pending editor copy has completed.
func TestHomeProjectRoutesTwoProcessesPendingInstall(t *testing.T) {
	var routes []string
	for _, consumer := range []string{"Compile", "CompileBatch", "ServerModules", "ServerVendor", "ServerShims", "ServerRuntime", "DevLWC", "DevVF"} {
		for _, selector := range []string{"default", "GLADE_HOME", "GLADE_ROOT", "GLADE_HOME_relative", "GLADE_ROOT_relative", "GLADE_HOME_parent", "GLADE_ROOT_parent", "GLADE_HOME_symlink", "GLADE_ROOT_symlink", "GLADE_HOME_case", "GLADE_ROOT_case"} {
			routes = append(routes, consumer+"/"+selector)
		}
	}
	toolchaininstall.PendingConsumers(t, routes, func(route string) error { return runHomeProjectConsumer(t, route) })
}

func runHomeProjectConsumer(t *testing.T, route string) error {
	if route == "install" {
		src, err := os.Getwd()
		if err != nil {
			return err
		}
		return gladehome.InstallFrom(src)
	}
	consumer, selector, _ := strings.Cut(route, "/")
	toolchaininstall.SelectShare(t, selector)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/ready/ready.js"), `export default class Ready {}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/ready/ready.html"), `<template>Ready</template>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/ready/ready.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	switch consumer {
	case "Compile":
		_, err = compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "out")})
	case "CompileBatch":
		_, err = compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "out")})
	case "ServerModules", "ServerVendor", "ServerShims", "ServerRuntime":
		source, sourceErr := server.NewSourceMetadataFromProject(p)
		if sourceErr != nil {
			return sourceErr
		}
		org := storage.NewOrgState()
		handler := server.NewWithSource(&org, source)
		defer handler.ResetLightningCache()
		endpoint := map[string]string{"ServerModules": "/lightning/modules/c/ready/ready.js", "ServerVendor": "/lightning/vendor/engine-dom.js", "ServerShims": "/lightning/shims/core/wire-adapter.js", "ServerRuntime": "/lightning/runtime/shell/app.mjs"}[consumer]
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if rec.Code != http.StatusOK {
			return fmt.Errorf("%s: HTTP %d: %s", consumer, rec.Code, rec.Body)
		}
	case "DevLWC", "DevVF":
		ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
		defer cancel()
		ready := filepath.Join(root, "ready.json")
		args := []string{"--project", root, "--addr", "127.0.0.1:0", "--ready-file", ready, "--progress-json"}
		done := make(chan error, 1)
		var messages bytes.Buffer
		go func() {
			if consumer == "DevLWC" {
				done <- runDevLWC(ctx, args, &messages, &messages)
			} else {
				done <- runDevVF(ctx, args, &messages, &messages)
			}
		}()
		for {
			if _, statErr := os.Stat(ready); statErr == nil {
				// Check readiness at the actual command boundary, before cancellation.
				_, err = os.Stat(filepath.Join(gladehome.UserShareDir(), "editor/vscode-glade.vsix"))
				cancel()
				serveErr := <-done
				if err != nil {
					return fmt.Errorf("%s ready before installation completed: %w", consumer, err)
				}
				if strings.Contains(messages.String(), "LWC toolchain unavailable") || !strings.Contains(messages.String(), "LWC toolchain ready") {
					return fmt.Errorf("%s did not report a ready toolchain: %s", consumer, &messages)
				}
				return serveErr
			}
			select {
			case err := <-done:
				return fmt.Errorf("%s exited without readiness: %v", consumer, err)
			case <-time.After(5 * time.Millisecond):
			}
		}
	default:
		return fmt.Errorf("unknown consumer %q", consumer)
	}
	return err
}

func TestHomeTestDaemonIndependentOfPendingInstall(t *testing.T) {
	toolchaininstall.PendingIndependentConsumer(t, "test_daemon", func(route string) error {
		if route == "install" {
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			return gladehome.InstallFrom(src)
		}
		toolchaininstall.SelectShare(t, "GLADE_HOME")
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ReadyTest.cls"), `@isTest private class ReadyTest { @isTest static void ready() { System.assertEquals(2,1+1); } }`)
		stop := startDaemonEquivalenceServer(t, root)
		defer stop()
		response, err := testdaemon.RunV1(t.Context(), testdaemon.ServeSocketPath(root), testdaemon.RunRequestV1{SelectedClasses: []string{"ReadyTest"}, Parallelism: 1, NoDiskCache: true})
		if err != nil {
			return err
		}
		if !response.OK || response.Run == nil || response.Run.Summary().Total != 1 || response.Run.Summary().Passed != 1 {
			return fmt.Errorf("daemon run failed: %+v", response)
		}
		return nil
	})
}

func TestHomeCLIStatusRoutesTwoProcessesPendingInstall(t *testing.T) {
	var routes []string
	for _, command := range []string{"ToolchainStatusCLI", "DoctorCLI"} {
		for _, selector := range []string{"default", "GLADE_HOME", "GLADE_ROOT", "GLADE_HOME_relative", "GLADE_ROOT_relative", "GLADE_HOME_parent", "GLADE_ROOT_parent", "GLADE_HOME_symlink", "GLADE_ROOT_symlink", "GLADE_HOME_case", "GLADE_ROOT_case"} {
			routes = append(routes, command+"/"+selector)
		}
	}
	toolchaininstall.PendingConsumers(t, routes, func(route string) error { return runHomeCLIStatus(t, route) })
}

func runHomeCLIStatus(t *testing.T, route string) error {
	if route == "install" {
		src, err := os.Getwd()
		if err != nil {
			return err
		}
		return gladehome.InstallFrom(src)
	}
	command, selector, _ := strings.Cut(route, "/")
	toolchaininstall.SelectShare(t, selector)
	args := []string{"toolchain", "status", "--json"}
	if command == "DoctorCLI" {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  root: .\n")
		args = []string{"doctor", "--project", root, "--json"}
	}
	var output bytes.Buffer
	if code := Run(t.Context(), args, &output, &output); code != 0 {
		return fmt.Errorf("%s exited %d: %s", command, code, &output)
	}
	return nil
}

func TestHomeProjectRoutesCompletedReadOnlyHome(t *testing.T) {
	toolchaininstall.ReadOnlyCompleted(t, []string{"Compile", "CompileBatch", "ServerModules", "ServerVendor", "ServerShims", "ServerRuntime", "DevLWC", "DevVF"}, runHomeProjectConsumer)
}

func TestHomeCLIStatusRoutesCompletedReadOnlyHome(t *testing.T) {
	toolchaininstall.ReadOnlyCompleted(t, []string{"ToolchainStatusCLI", "DoctorCLI"}, runHomeCLIStatus)
}
