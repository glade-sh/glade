package project

import (
	"os"
	"path/filepath"
	"testing"
)

// apextest api68_native flow FLOW-DEPLOY-67-USERMODE rejects UserMode at
// Flow API 67.0; FLOW-DEPLOY-68 deploys it at 68.0. A default-mode Flow at 67.0
// deploys (FLOW-DEPLOY-67). A Flow without apiVersion was not captured.
func TestFlowRunModeRejections(t *testing.T) {
	root := t.TempDir()
	flows := map[string]string{
		"User67.flow-meta.xml":    `<Flow><apiVersion>67.0</apiVersion><runInMode>UserMode</runInMode></Flow>`,
		"User68.flow-meta.xml":    `<Flow><apiVersion>68.0</apiVersion><runInMode>UserMode</runInMode></Flow>`,
		"Default67.flow-meta.xml": `<Flow><apiVersion>67.0</apiVersion></Flow>`,
		"System67.flow-meta.xml":  `<Flow><apiVersion>67.0</apiVersion><runInMode>SystemModeWithoutSharing</runInMode></Flow>`,
		"NoVersion.flow-meta.xml": `<Flow><runInMode>UserMode</runInMode></Flow>`,
	}
	p := Project{Root: root}
	for name, body := range flows {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		p.FlowFiles = append(p.FlowFiles, path)
	}
	rejections := FlowRunModeRejections(p, nil)
	if len(rejections) != 1 || filepath.Base(rejections[0].File) != "User67.flow-meta.xml" {
		t.Fatalf("rejections = %+v", rejections)
	}
	want := `We can't save the "User67" flow. The Explicit User Mode run mode requires API version 68 or later. Select API version 68 or later, or choose a different run mode.`
	if rejections[0].Message != want {
		t.Fatalf("message = %q", rejections[0].Message)
	}
	if shapes := FlowRunModeShapes(p, nil); len(shapes) != 4 {
		t.Fatalf("shapes = %v", shapes)
	}
}
