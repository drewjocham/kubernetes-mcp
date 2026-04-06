package middleware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// KubeConfig holds Kubernetes configuration
type KubeConfig struct {
	Config    *rest.Config
	Clientset *kubernetes.Clientset
	Context   string
	Namespace string
}

// KubeConfigMiddlewareConfig holds configuration for kubeconfig middleware
type KubeConfigMiddlewareConfig struct {
	KubeconfigPath string
	Context        string
	Namespace      string
	Required       bool // Whether kubeconfig is required for the command
}

// DefaultKubeConfigMiddlewareConfig returns default kubeconfig middleware configuration
func DefaultKubeConfigMiddlewareConfig() KubeConfigMiddlewareConfig {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return KubeConfigMiddlewareConfig{
		KubeconfigPath: kubeconfig,
		Context:        "",
		Namespace:      "default",
		Required:       false,
	}
}

// NewKubeConfigMiddleware creates middleware that loads Kubernetes configuration
func NewKubeConfigMiddleware(config KubeConfigMiddlewareConfig) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		// Add flags for kubeconfig
		cmd.PersistentFlags().StringVar(&config.KubeconfigPath, "kubeconfig", config.KubeconfigPath, "path to kubeconfig file")
		cmd.PersistentFlags().StringVar(&config.Context, "context", config.Context, "kubeconfig context to use")
		cmd.PersistentFlags().StringVar(&config.Namespace, "namespace", config.Namespace, "kubernetes namespace to use")

		// Store config in command context for later use
		originalRun := cmd.RunE
		if originalRun != nil {
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				// Load kubeconfig
				kubeConfig, err := loadKubeConfig(config)
				if err != nil {
					if config.Required {
						return fmt.Errorf("failed to load kubeconfig: %w", err)
					}
					// If not required, just log warning
					fmt.Fprintf(os.Stderr, "Warning: Could not load kubeconfig: %v\n", err)
				}

				// Store in command context
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				if kubeConfig != nil {
					ctx = context.WithValue(ctx, "kubeconfig", kubeConfig)
					cmd.SetContext(ctx)
				}

				return originalRun(cmd, args)
			}
		}
	}
}

// loadKubeConfig loads Kubernetes configuration
func loadKubeConfig(config KubeConfigMiddlewareConfig) (*KubeConfig, error) {
	// Use in-cluster config if available
	if restConfig, err := rest.InClusterConfig(); err == nil {
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("create clientset from in-cluster config: %w", err)
		}

		return &KubeConfig{
			Config:    restConfig,
			Clientset: clientset,
			Context:   "in-cluster",
			Namespace: config.Namespace,
		}, nil
	}

	// Load from kubeconfig file
	if config.KubeconfigPath == "" {
		return nil, fmt.Errorf("kubeconfig path not specified and not running in-cluster")
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = config.KubeconfigPath

	overrides := &clientcmd.ConfigOverrides{}
	if config.Context != "" {
		overrides.CurrentContext = config.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("create rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	// Get namespace from config
	ns, _, err := clientConfig.Namespace()
	if err != nil {
		ns = config.Namespace
	}

	return &KubeConfig{
		Config:    restConfig,
		Clientset: clientset,
		Context:   config.Context,
		Namespace: ns,
	}, nil
}

// GetKubeConfigFromContext retrieves kubeconfig from command context
func GetKubeConfigFromContext(cmd *cobra.Command) *KubeConfig {
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}

	if kubeConfig, ok := ctx.Value("kubeconfig").(*KubeConfig); ok {
		return kubeConfig
	}

	return nil
}
