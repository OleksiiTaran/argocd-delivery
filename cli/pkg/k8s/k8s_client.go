package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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

	secretName := "grafana-alloy"
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

%s kubectl port-forward svc/grafana -n monitoring 3000:80`,
		pterm.FgGreen.Sprint("URL:"),
		pterm.FgGreen.Sprint("User:"),
		pterm.FgGreen.Sprint("Pass:"), pterm.FgCyan.Sprint(password),
		pterm.FgGray.Sprint("To access, run:"),
	)

	panel.Println(content)
}
