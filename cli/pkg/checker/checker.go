package checker

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/pterm/pterm"
)

var RequiredTools = []string{
	"docker",
	"kind",
	"terragrunt",
	"terraform",
	"kubectl",
}

func CheckDependencies() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("native Windows is not supported. Please run this tool inside WSL2")
	}

	pterm.Info.Printf("Detecting OS: %s\n", runtime.GOOS)

	spinner, _ := pterm.DefaultSpinner.Start("Checking required dependencies...")

	missingTools := []string{}

	for _, tool := range RequiredTools {
		_, err := exec.LookPath(tool)
		if err != nil {
			missingTools = append(missingTools, tool)
			pterm.Warning.Printf("%s is missing\n", tool)
		} else {
			// Використовуємо адаптивний FgGreen замість стандартного виводу
			pterm.Success.Printf("%s is installed\n", tool)
		}
	}

	if len(missingTools) > 0 {
		spinner.Warning("Some dependencies are missing!")
		return handleMissingTools(missingTools)
	}

	spinner.Success("All dependencies are met!")
	return nil
}

func handleMissingTools(tools []string) error {
	pterm.Warning.Printf("Missing tools: %v\n", tools)

	result, err := pterm.DefaultInteractiveConfirm.Show("Would you like to attempt automatic installation?")
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if result {
		pterm.Println()
		for _, tool := range tools {
			installTool(tool)
		}

		pterm.Info.Println("Re-checking dependencies after installation...")
		return verifyPostInstall(tools)
	}

	return fmt.Errorf("installation aborted by user. Please install the missing tools manually")
}

func installTool(tool string) {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Attempting to install %s...", tool))
	osType := runtime.GOOS

	switch osType {
	case "darwin":
		installMac(tool, spinner)
	case "linux":
		installLinux(tool, spinner)
	default:
		spinner.Warning(fmt.Sprintf("Automatic installation for %s is not supported yet.", osType))
	}
}

func installMac(tool string, spinner *pterm.SpinnerPrinter) {
	if _, err := exec.LookPath("brew"); err != nil {
		spinner.Fail("Homebrew is not installed. Please install Homebrew first: https://brew.sh/")
		return
	}

	var cmd *exec.Cmd
	switch tool {
	case "docker":
		cmd = exec.Command("brew", "install", "--cask", "docker")
	case "terraform":
		// Підключаємо офіційний репозиторій HashiCorp через зміни в ліцензії
		_ = exec.Command("brew", "tap", "hashicorp/tap").Run()
		cmd = exec.Command("brew", "install", "hashicorp/tap/terraform")
	default:
		cmd = exec.Command("brew", "install", tool)
	}

	if err := cmd.Run(); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to install %s: %v", tool, err))
	} else {
		spinner.Success(fmt.Sprintf("Successfully installed %s", tool))
	}
}

func installLinux(tool string, spinner *pterm.SpinnerPrinter) {
	if tool == "docker" {
		spinner.Warning("Docker requires manual installation on Linux (GPG keys, usermod). Please check docs.docker.com")
		return
	}

	if _, err := exec.LookPath("snap"); err == nil {
		cmd := exec.Command("sudo", "snap", "install", tool, "--classic")
		if err := cmd.Run(); err != nil {
			spinner.Fail(fmt.Sprintf("Failed to install %s via snap", tool))
		} else {
			spinner.Success(fmt.Sprintf("Successfully installed %s", tool))
		}
	} else {
		spinner.Fail(fmt.Sprintf("Snap is not available. Please install %s manually.", tool))
	}
}

func verifyPostInstall(tools []string) error {
	stillMissing := []string{}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			stillMissing = append(stillMissing, tool)
		}
	}

	if len(stillMissing) > 0 {
		return fmt.Errorf("some tools could not be installed automatically: %v", stillMissing)
	}

	pterm.Success.Println("All dependencies successfully installed!")
	return nil
}

func UninstallTools() error {
	toolsToRemove := []string{
		"kind",
		"terragrunt",
		"terraform",
		"kubectl",
	}

	pterm.Warning.Println("Starting removal of CLI tools...")

	for _, tool := range toolsToRemove {
		uninstallTool(tool)
	}

	pterm.Success.Println("Cleanup of CLI tools complete!")
	return nil
}

func uninstallTool(tool string) {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Attempting to uninstall %s...", tool))
	osType := runtime.GOOS

	switch osType {
	case "darwin":
		cmd := exec.Command("brew", "uninstall", tool)
		if err := cmd.Run(); err != nil {
			// Якщо brew не зміг видалити (наприклад, kubectl від Docker Desktop),
			// ми не падаємо, а просто інформуємо користувача.
			spinner.Warning(fmt.Sprintf("%s was not managed by Homebrew (might be part of Docker Desktop). Skipped.", tool))
		} else {
			spinner.Success(fmt.Sprintf("Successfully uninstalled %s", tool))
		}
	case "linux":
		if _, err := exec.LookPath("snap"); err == nil {
			cmd := exec.Command("sudo", "snap", "remove", tool)
			if err := cmd.Run(); err != nil {
				spinner.Fail(fmt.Sprintf("Failed to uninstall %s via snap.", tool))
			} else {
				spinner.Success(fmt.Sprintf("Successfully uninstalled %s", tool))
			}
		} else {
			spinner.Fail(fmt.Sprintf("Snap is not available. Please uninstall %s manually.", tool))
		}
	default:
		spinner.Warning(fmt.Sprintf("Automatic uninstallation for %s is not supported.", osType))
	}
}
