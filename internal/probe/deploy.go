package probe

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Deployer handles deploying the probe SFDX project to a Salesforce org,
// assigning permission sets, and seeding data.
type Deployer struct {
	ProbeDir string
	OrgAlias string
}

// NewDeployer creates a Deployer for the given probe project and org.
func NewDeployer(probeDir, orgAlias string) *Deployer {
	return &Deployer{ProbeDir: probeDir, OrgAlias: orgAlias}
}

// Deploy runs the full deployment pipeline: project deploy, permset assign, data seed.
func (d *Deployer) Deploy(ctx context.Context, w io.Writer) error {
	fmt.Fprintln(w, "Deploying probe project to", d.OrgAlias)

	// 1. Deploy project
	if err := d.runCmd(ctx, w, "sf", "project", "deploy", "start", "--target-org", d.OrgAlias, "--ignore-conflicts", "--json"); err != nil {
		return fmt.Errorf("deploy project: %w", err)
	}
	fmt.Fprintln(w, "Project deployed successfully.")

	// 2. Assign permission set (ignore if already assigned)
	permsetOut, permsetErr := d.runCmdOutput(ctx, "sf", "org", "assign", "permset", "--name", "ProbeDataAccess", "--target-org", d.OrgAlias)
	fmt.Fprintln(w, permsetOut)
	if permsetErr != nil && !strings.Contains(permsetOut, "Duplicate PermissionSetAssignment") {
		return fmt.Errorf("assign permset: %w", permsetErr)
	}
	fmt.Fprintln(w, "Permission set assigned.")

	// 3. Clear existing probe records, then seed fresh data
	dataFile := filepath.Join(d.ProbeDir, "data", "ProbeTestObjects.json")
	if _, err := os.Stat(dataFile); err == nil {
		if err := d.ResetProbeData(ctx, w); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(w, "No data file found at", dataFile, "- skipping seed.")
	}

	fmt.Fprintln(w, "Deploy complete.")
	return nil
}

// ResetProbeData clears and re-seeds records used by data-sensitive probes.
func (d *Deployer) ResetProbeData(ctx context.Context, w io.Writer) error {
	if _, err := os.Stat(filepath.Join(d.ProbeDir, "data", "cleanup.apex")); err != nil {
		return fmt.Errorf("probe cleanup script missing: %w", err)
	}
	if _, err := os.Stat(filepath.Join(d.ProbeDir, "data", "ProbeTestObjects.json")); err != nil {
		return fmt.Errorf("probe seed data missing: %w", err)
	}
	fmt.Fprintln(w, "Resetting probe data.")
	if _, err := d.runCmdOutput(ctx, "sf", "apex", "run", "--target-org", d.OrgAlias, "--file", "data/cleanup.apex"); err != nil {
		return fmt.Errorf("cleanup probe data: %w", err)
	}
	if err := d.runCmd(ctx, w, "sf", "data", "import", "tree", "--files", "data/ProbeTestObjects.json", "--target-org", d.OrgAlias); err != nil {
		return fmt.Errorf("seed data: %w", err)
	}
	fmt.Fprintln(w, "Data seeded.")
	return nil
}

func (d *Deployer) runCmd(ctx context.Context, w io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = d.ProbeDir
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

func (d *Deployer) runCmdOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = d.ProbeDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
