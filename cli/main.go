package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/OleksiiTaran/argocd-delivery/cli/pkg/checker"
	"github.com/OleksiiTaran/argocd-delivery/cli/pkg/k8s"
	"github.com/OleksiiTaran/argocd-delivery/cli/pkg/runner"
)

var rootCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Sandbox CLI - a tool to automate local Kubernetes cluster setup",
	Long:  `Sandbox CLI orchestrates the deployment of a local environment using Kind, Terraform/Terragrunt, and ArgoCD.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Display help menu if no subcommands are provided by the user
		cmd.Help()
	},
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check system dependencies",
	Long:  `Detects the operating system and checks if all required tools (Docker, Kind, Terragrunt, etc.) are installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := checker.CheckDependencies()
		if err != nil {
			pterm.Error.Printf("Dependency check failed: %v\n", err)
			os.Exit(1)
		}
	},
}

// upCmd represents the command to spin up the entire sandbox
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Spin up the local Kubernetes sandbox",
	Long:  `Validates dependencies and provisions the infrastructure using Terragrunt.`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("SANDBOX CLI DEPLOYMENT")
		pterm.Info.Println("Step 1: Running pre-flight checks...")
		if err := checker.CheckDependencies(); err != nil {
			pterm.Error.Printf("\nPre-flight check failed: %v\n", err)
			os.Exit(1)
		}

		pterm.Println()

		repoPromt := &survey.Select{
			Message: "Which infrastructure configuration would you like to use?",
			Options: []string{
				"Default (Pull from OleksiiTaran/argocd-delivery)",
				"Custom (Provide local path)",
			},
		}
		var configChoice string
		if err := survey.AskOne(repoPromt, &configChoice); err != nil {
			pterm.Error.Println("Input cancelled.")
			os.Exit(1)
		}

		var tgDir string
		var appOfAppsManifestPath string

		if configChoice == "Default (Pull from OleksiiTaran/argocd-delivery)" {

			tempDir, err := os.MkdirTemp("", "sandbox-infra-")
			if err != nil {
				pterm.Error.Printf("Failed to create temporary directory: %v\n", err)
				os.Exit(1)
			}
			defer os.RemoveAll(tempDir)

			spinner, _ := pterm.DefaultSpinner.Start("Cloning remote infrastructure repository...")

			cloneCmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/OleksiiTaran/argocd-delivery.git", tempDir)
			if err := cloneCmd.Run(); err != nil {
				spinner.Fail(fmt.Sprintf("Failed to clone repository: %v", err))
				os.Exit(1)
			}
			spinner.Success("Repository successfully cloned into temporary directory!")

			tgDir = filepath.Join(tempDir, "terragrunt", "environments", "dev")
			appOfAppsManifestPath = filepath.Join(tempDir, "argo-apps", "root-app.yaml")
		} else {
			pathPrompt := &survey.Input{
				Message: "Enter the local path to your Terragrunt environment:",
				Default: "../terragrunt/environments/dev",
			}
			if err := survey.AskOne(pathPrompt, &tgDir); err != nil {
				pterm.Error.Println("Input cancelled.")
				os.Exit(1)
			}

			manifestPrompt := &survey.Input{
				Message: "Enter the local path to your ArgoCD app-of-apps manifest:",
				Default: filepath.Clean(filepath.Join(tgDir, "..", "..", "..", "argo-apps", "root-app.yaml")),
			}
			if err := survey.AskOne(manifestPrompt, &appOfAppsManifestPath); err != nil {
				pterm.Error.Println("Input cancelled.")
				os.Exit(1)
			}
		}

		var argoRepoURL string

		argoPrompt := &survey.Input{
			Message: "Enter target Git repository URL for ArgoCD GitOps (Press Enter for default):",
			Default: "https://github.com/OleksiiTaran/argocd-delivery.git",
		}
		if err := survey.AskOne(argoPrompt, &argoRepoURL); err != nil {
			pterm.Error.Println("Input cancelled.")
			os.Exit(1)
		}

		os.Setenv("TF_VAR_argo_repo_url", argoRepoURL)

		pterm.Success.Println()
		pterm.Info.Printfln("Step 2: Provisioning Cluster and Ingress...")
		if err := runner.ApplyInfrastructure(tgDir); err != nil {
			pterm.Error.Printf("\nInfrastructure setup failed: %v\n", err)
			os.Exit(1)
		}

		pterm.Println()
		pterm.Info.Println("Step 3: Bootstrapping ArgoCD App-of-Apps...")
		if err := k8s.BootstrapArgoCDAppOfApps(appOfAppsManifestPath); err != nil {
			pterm.Error.Printf("\nArgoCD bootstrap failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var credentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Fetch login credentials for deployed applications",
	Long:  `Queries the Kubernetes API via client-go to retrieve and decode passwords for ArgoCD, Grafana, and Loki.`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgMagenta)).WithTextStyle(pterm.NewStyle(pterm.FgWhite)).Println("APPLICATION CREDENTIALS")

		if err := k8s.FetchCredentials(); err != nil {
			pterm.Error.Printf("Failed to fetch credentials: %v\n", err)
			os.Exit(1)
		}
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Tear down the local Kubernetes sandbox",
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgRed)).WithTextStyle(pterm.NewStyle(pterm.FgWhite)).Println("SANDBOX CLI TEARDOWN")

		confirm, err := pterm.DefaultInteractiveConfirm.Show("Are you sure you want to destroy the entire infrastructure?")
		if err != nil || !confirm {
			pterm.Info.Println("Teardown cancelled.")
			os.Exit(0)
		}

		var tgDir string
		pathPrompt := &survey.Input{
			Message: "Enter the local path to your Terragrunt environment to destroy:",
			Default: "../terragrunt/environments/dev",
		}
		if err := survey.AskOne(pathPrompt, &tgDir); err != nil {
			pterm.Error.Println("Input cancelled.")
			os.Exit(1)
		}

		pterm.Println()
		pterm.Info.Printf("Executing destroy in directory: %s\n", tgDir)

		if err := runner.DestroyInfrastructure(tgDir); err != nil {
			pterm.Error.Printf("Infrastructure teardown failed: %v\n", err)
			os.Exit(1)
		}

		pterm.Println()
		removeTools, err := pterm.DefaultInteractiveConfirm.Show("Would you also like to uninstall the CLI tools (Kind, Terragrunt, Terraform, Kubectl)? Docker will NOT be removed.")
		if err == nil && removeTools {
			pterm.Println()
			if err := checker.UninstallTools(); err != nil {
				pterm.Error.Printf("Tool uninstallation encountered an issue: %v\n", err)
			}
		} else {
			pterm.Info.Println("CLI tools were kept on your system.")
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(credentialsCmd)
	rootCmd.AddCommand(destroyCmd)

	pterm.DefaultSpinner.TimerStyle = pterm.NewStyle(pterm.FgDarkGray)
	pterm.DefaultSpinner.Style = pterm.NewStyle(pterm.FgCyan)
	pterm.DefaultSpinner.MessageStyle = pterm.NewStyle(pterm.FgDefault)
}

func main() {
	// Execute the root command and handle potential errors gracefully
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
