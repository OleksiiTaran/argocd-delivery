package runner

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pterm/pterm"
)

// ApplyInfrastructure runs Terragrunt to provision the cluster and baseline infrastructure
func ApplyInfrastructure(workingDir string) error {
	pterm.Info.Println("🚀 Starting infrastructure provisioning via Terragrunt...")

	// Command equivalent to: terragrunt run-all apply -auto-approve --terragrunt-non-interactive
	cmd := exec.Command("terragrunt", "apply", "--non-interactive", "--", "-auto-approve")

	// Set the directory where Terragrunt should execute
	cmd.Dir = workingDir

	// Stream standard output and error directly to the user's terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("infrastructure provisioning failed: %w", err)
	}

	pterm.Success.Println("✅ Infrastructure successfully provisioned!")
	return nil
}

// DestroyInfrastructure runs Terragrunt to tear down the cluster
func DestroyInfrastructure(workingDir string) error {
	pterm.Warning.Println("Starting infrastructure teardown via Terragrunt...")

	cmd := exec.Command("terragrunt", "destroy", "--non-interactive", "--", "-auto-approve")

	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("infrastructure teardown failed: %w", err)
	}

	pterm.Success.Println("Infrastructure successfully destroyed via Terragrunt!")

	clusterName := "local-sandbox"

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Forcefully deleting Kind cluster '%s'...", clusterName))

	kindCmd := exec.Command("kind", "delete", "cluster", "--name", clusterName)
	if err := kindCmd.Run(); err != nil {
		// Якщо кластера вже не існує, не падаємо, а просто попереджаємо
		spinner.Warning(fmt.Sprintf("Could not delete Kind cluster (it might already be destroyed): %v", err))
	} else {
		spinner.Success(fmt.Sprintf("Kind cluster '%s' successfully deleted!", clusterName))
	}

	return nil
}
