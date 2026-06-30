package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	"github.com/pterm/pterm"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type applicationManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

func getClient() (*kubernetes.Clientset, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	kubeconfig := filepath.Join(homeDir, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

func FetchCredentials() error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	pterm.Info.Println("Successfully connected to Kubernetes cluster.")
	pterm.Println()

	fetchArgoCD(clientset)
	pterm.Println()

	fetchGrafana(clientset)

	return nil
}

func fetchArgoCD(clientset *kubernetes.Clientset) {
	panel := pterm.DefaultBox.WithTitle("🚢 ArgoCD Dashboard").WithTitleTopLeft()

	secretName := "argocd-initial-admin-secret"
	namespace := "argocd"

	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		panel.Println(pterm.Warning.Sprintf("Could not fetch ArgoCD secret. Is it deployed?\nError: %v", err))
		return
	}

	password := string(secret.Data["password"])

	// Змінено LightCyan на адаптивний FgGreen для кращого контрасту на світлих темах
	content := fmt.Sprintf(`%s http://localhost:8080
%s admin
%s %s

%s kubectl port-forward svc/argocd-server -n argocd 8080:443`,
		pterm.FgGreen.Sprint("URL:"),
		pterm.FgGreen.Sprint("User:"),
		pterm.FgGreen.Sprint("Pass:"), pterm.FgCyan.Sprint(password),
		pterm.FgGray.Sprint("To access, run:"),
	)

	panel.Println(content)
}

func fetchGrafana(clientset *kubernetes.Clientset) {
	panel := pterm.DefaultBox.WithTitle("📊 Grafana & Loki").WithTitleTopLeft()

	secretName := "prometheus-stack-grafana"
	namespace := "monitoring"

	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		panel.Println(pterm.Warning.Sprintf("Could not fetch Grafana secret.\nError: %v", err))
		return
	}

	password := string(secret.Data["admin-password"])

	// Аналогічно замінено на адаптивний FgGreen та FgCyan
	content := fmt.Sprintf(`%s http://localhost:3000
%s admin
%s %s

%s kubectl port-forward svc/prometheus-stack-grafana -n monitoring 3000:80`,
		pterm.FgGreen.Sprint("URL:"),
		pterm.FgGreen.Sprint("User:"),
		pterm.FgGreen.Sprint("Pass:"), pterm.FgCyan.Sprint(password),
		pterm.FgGray.Sprint("To access, run:"),
	)

	panel.Println(content)
}

func BootstrapArgoCDAppOfApps(manifestPath string) error {
	absManifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute manifest path: %w", err)
	}

	if _, err := os.Stat(absManifestPath); err != nil {
		return fmt.Errorf("app-of-apps manifest was not found at %s: %w", absManifestPath, err)
	}

	appName, appNamespace, err := getManifestApplicationIdentity(absManifestPath)
	if err != nil {
		return err
	}

	if err := waitForResource("namespace", "argocd", "", 2*time.Minute); err != nil {
		return fmt.Errorf("argocd namespace is not ready: %w", err)
	}

	if err := waitForResource("crd", "applications.argoproj.io", "", 2*time.Minute); err != nil {
		return fmt.Errorf("argocd application CRD is not ready: %w", err)
	}

	if _, err := runKubectl("apply", "-f", absManifestPath); err != nil {
		return fmt.Errorf("failed to apply app-of-apps manifest: %w", err)
	}

	if err := waitForApplicationReady(appNamespace, appName, 3*time.Minute); err != nil {
		return fmt.Errorf("app-of-apps was created but did not become ready: %w", err)
	}

	pterm.Success.Printf("ArgoCD bootstrap completed: application %s/%s is Synced and Healthy.\n", appNamespace, appName)
	return nil
}

func getManifestApplicationIdentity(manifestPath string) (string, string, error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read app-of-apps manifest: %w", err)
	}

	var manifest applicationManifest
	if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", "", fmt.Errorf("failed to parse app-of-apps manifest: %w", err)
	}

	if manifest.Kind != "Application" {
		return "", "", fmt.Errorf("manifest kind must be Application, got %q", manifest.Kind)
	}

	if strings.TrimSpace(manifest.Metadata.Name) == "" {
		return "", "", fmt.Errorf("manifest metadata.name is required")
	}

	namespace := strings.TrimSpace(manifest.Metadata.Namespace)
	if namespace == "" {
		namespace = "argocd"
	}

	return strings.TrimSpace(manifest.Metadata.Name), namespace, nil
}

func waitForResource(kind, name, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		args := []string{"get", kind, name}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}

		if _, err := runKubectl(args...); err == nil {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timed out waiting for %s/%s", kind, name)
}

func waitForApplicationReady(namespace, appName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastSyncStatus := "<unknown>"
	lastHealthStatus := "<unknown>"

	for time.Now().Before(deadline) {
		syncStatus, syncErr := runKubectl(
			"get", "application", appName,
			"-n", namespace,
			"-o", "jsonpath={.status.sync.status}",
		)

		healthStatus, healthErr := runKubectl(
			"get", "application", appName,
			"-n", namespace,
			"-o", "jsonpath={.status.health.status}",
		)

		if syncErr == nil {
			lastSyncStatus = strings.TrimSpace(syncStatus)
		}

		if healthErr == nil {
			lastHealthStatus = strings.TrimSpace(healthStatus)
		}

		if lastSyncStatus == "Synced" && lastHealthStatus == "Healthy" {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timed out waiting for status (sync=%s, health=%s)", lastSyncStatus, lastHealthStatus)
}

func runKubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		if trimmedOutput == "" {
			return "", fmt.Errorf("kubectl %v failed: %w", args, err)
		}

		return "", fmt.Errorf("kubectl %v failed: %w: %s", args, err, trimmedOutput)
	}

	return trimmedOutput, nil
}
