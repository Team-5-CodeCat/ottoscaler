package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	GRPC       GRPCConfig       `yaml:"grpc"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Worker     WorkerConfig     `yaml:"worker"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port            int    `yaml:"port"`
	OttoHandlerHost string `yaml:"otto_handler_host"`
	MockMode        bool   `yaml:"mock_mode"` // Mock mode for development/testing
}

// KubernetesConfig holds Kubernetes cluster configuration
type KubernetesConfig struct {
	Namespace      string `yaml:"namespace"`
	ServiceAccount string `yaml:"service_account"`
}

// WorkerConfig holds Worker Pod configuration
type WorkerConfig struct {
	Image       string            `yaml:"image"`
	CPULimit    string            `yaml:"cpu_limit"`
	MemoryLimit string            `yaml:"memory_limit"`
	Labels      map[string]string `yaml:"labels"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load loads configuration from YAML file and environment variables
func Load(configPath string) (*Config, error) {
	// Load base configuration from YAML file
	config, err := loadFromYAML(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load YAML config: %w", err)
	}

	// Override with environment variables if present
	overrideWithEnv(config)

	// Validate configuration
	if err := validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadFromEnv loads configuration entirely from environment variables
func LoadFromEnv() (*Config, error) {
	config := &Config{
		GRPC: GRPCConfig{
			Port:            getEnvInt("GRPC_PORT", 9090),
			OttoHandlerHost: getEnv("OTTO_HANDLER_HOST", "otto-handler:8080"),
			MockMode:        getEnvBool("GRPC_MOCK_MODE", true), // Default to mock mode for safety
		},
		Kubernetes: KubernetesConfig{
			Namespace:      getEnv("NAMESPACE", "default"),
			ServiceAccount: getEnv("SERVICE_ACCOUNT", "ottoscaler"),
		},
		Worker: WorkerConfig{
			Image:       getEnv("OTTO_AGENT_IMAGE", "busybox:latest"),
			CPULimit:    getEnv("WORKER_CPU_LIMIT", "500m"),
			MemoryLimit: getEnv("WORKER_MEMORY_LIMIT", "128Mi"),
			Labels: map[string]string{
				"managed-by": "ottoscaler",
			},
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "text"),
		},
	}

	if err := validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadFromYAML loads configuration from a YAML file
func loadFromYAML(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// overrideWithEnv overrides YAML config with environment variables
func overrideWithEnv(config *Config) {
	// gRPC overrides
	if port := os.Getenv("GRPC_PORT"); port != "" {
		if portInt, err := strconv.Atoi(port); err == nil {
			config.GRPC.Port = portInt
		}
	}
	if host := os.Getenv("OTTO_HANDLER_HOST"); host != "" {
		config.GRPC.OttoHandlerHost = host
	}
	if mockMode := os.Getenv("GRPC_MOCK_MODE"); mockMode != "" {
		config.GRPC.MockMode = parseBool(mockMode)
	}

	// Kubernetes overrides
	if namespace := os.Getenv("NAMESPACE"); namespace != "" {
		config.Kubernetes.Namespace = namespace
	}
	if serviceAccount := os.Getenv("SERVICE_ACCOUNT"); serviceAccount != "" {
		config.Kubernetes.ServiceAccount = serviceAccount
	}

	// Worker overrides
	if image := os.Getenv("OTTO_AGENT_IMAGE"); image != "" {
		config.Worker.Image = image
	}
	if cpuLimit := os.Getenv("WORKER_CPU_LIMIT"); cpuLimit != "" {
		config.Worker.CPULimit = cpuLimit
	}
	if memoryLimit := os.Getenv("WORKER_MEMORY_LIMIT"); memoryLimit != "" {
		config.Worker.MemoryLimit = memoryLimit
	}

	// Logging overrides
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
}

// validate validates the configuration
func validate(config *Config) error {
	if config.GRPC.Port <= 0 || config.GRPC.Port > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", config.GRPC.Port)
	}

	if config.Kubernetes.Namespace == "" {
		return fmt.Errorf("kubernetes namespace cannot be empty")
	}

	if config.Worker.Image == "" {
		return fmt.Errorf("worker image cannot be empty")
	}

	return nil
}

// GetWorkerLabels returns worker pod labels with additional custom labels
func (c *Config) GetWorkerLabels(additionalLabels map[string]string) map[string]string {
	labels := make(map[string]string)

	// Copy base labels from config
	for k, v := range c.Worker.Labels {
		labels[k] = v
	}

	// Add additional labels
	for k, v := range additionalLabels {
		labels[k] = v
	}

	return labels
}

// GetGRPCAddr returns the gRPC server address
func (c *Config) GetGRPCAddr() string {
	return fmt.Sprintf(":%d", c.GRPC.Port)
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as integer with default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets environment variable as boolean with default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return parseBool(value)
	}
	return defaultValue
}

// parseBool parses a string to boolean
func parseBool(value string) bool {
	switch value {
	case "true", "TRUE", "True", "1", "yes", "YES", "Yes":
		return true
	case "false", "FALSE", "False", "0", "no", "NO", "No":
		return false
	default:
		return false
	}
}
