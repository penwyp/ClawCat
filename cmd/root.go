package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/internal"
	"github.com/penwyp/claudecat/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	logLevel string
	noColor  bool
	debug    bool
	verbose  bool
	// Run command flags moved to root
	runPaths      []string
	runPlan       string
	runRefresh    time.Duration
	runTheme      string
	runWatch      bool
	runBackground bool
	// New pricing and deduplication flags
	pricingSource      string
	pricingOffline     bool
	enableDeduplication bool
)

var rootCmd = &cobra.Command{
	Use:   "claudecat",
	Short: "Claude Code Usage Monitor",
	Long: `claudecat is a high-performance TUI application for monitoring Claude AI token usage and costs.

It provides real-time monitoring, session analysis, cost calculations, and data export
capabilities to help developers track their Claude API usage efficiently.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load and validate configuration
		cfg, err := loadConfiguration(cmd)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Override configuration with command line flags
		if err := applyRunFlags(cfg); err != nil {
			return fmt.Errorf("failed to apply command flags: %w", err)
		}

		// Apply debug flag if set from command line
		if debug {
			cfg.Debug.Enabled = true
			// Set log level to debug when debug flag is enabled
			cfg.App.LogLevel = "debug"
		}

		// Initialize global logger with debug mode support
		logging.InitLogger(cfg.App.LogLevel, cfg.App.LogFile, cfg.Debug.Enabled)

		// Create and run enhanced application
		app, err := internal.NewEnhancedApplication(cfg)
		if err != nil {
			return fmt.Errorf("failed to create enhanced application: %w", err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Starting claudecat enhanced TUI monitor...\n")
			fmt.Fprintf(os.Stderr, "Configuration: %+v\n", cfg)
		}

		return app.Run()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Disable help command by setting a hidden command that returns error
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("Error: unknown command \"help\" for \"claudecat\"\n")
			cmd.Printf("Run 'claudecat --help' for usage.\n")
		},
	})

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.claudecat.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Run command flags (now default behavior)
	rootCmd.Flags().StringSliceVarP(&runPaths, "paths", "p", nil, "data paths to monitor (can be specified multiple times)")
	rootCmd.Flags().StringVar(&runPlan, "plan", "", "subscription plan (free, pro, team)")
	rootCmd.Flags().DurationVarP(&runRefresh, "refresh", "r", 0, "refresh interval (e.g., 1s, 500ms)")
	rootCmd.Flags().StringVarP(&runTheme, "theme", "t", "", "UI theme (dark, light, high-contrast)")
	rootCmd.Flags().BoolVarP(&runWatch, "watch", "w", false, "enable file watching for real-time updates")
	rootCmd.Flags().BoolVar(&runBackground, "background", false, "run in background mode (minimal UI)")
	
	// Pricing and deduplication flags
	rootCmd.Flags().StringVar(&pricingSource, "pricing-source", "", "pricing source (default, litellm)")
	rootCmd.Flags().BoolVar(&pricingOffline, "pricing-offline", false, "use cached pricing data for offline mode")
	rootCmd.Flags().BoolVar(&enableDeduplication, "deduplication", false, "enable deduplication of entries across all files")

	// Bind flags to viper
	if err := viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		// During initialization, print to stderr
		fmt.Fprintf(os.Stderr, "Failed to bind log-level flag: %v\n", err)
	}
	if err := viper.BindPFlag("ui.no_color", rootCmd.PersistentFlags().Lookup("no-color")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind no-color flag: %v\n", err)
	}
	if err := viper.BindPFlag("debug.enabled", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind debug flag: %v\n", err)
	}
	if err := viper.BindPFlag("log.verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind verbose flag: %v\n", err)
	}

	// Bind run command flags to viper
	if err := viper.BindPFlag("app.data_paths", rootCmd.Flags().Lookup("paths")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind paths flag: %v\n", err)
	}
	if err := viper.BindPFlag("app.subscription_plan", rootCmd.Flags().Lookup("plan")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind plan flag: %v\n", err)
	}
	if err := viper.BindPFlag("app.refresh_interval", rootCmd.Flags().Lookup("refresh")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind refresh flag: %v\n", err)
	}
	if err := viper.BindPFlag("ui.theme", rootCmd.Flags().Lookup("theme")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind theme flag: %v\n", err)
	}
	if err := viper.BindPFlag("fileio.watch_enabled", rootCmd.Flags().Lookup("watch")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind watch flag: %v\n", err)
	}
	if err := viper.BindPFlag("app.background_mode", rootCmd.Flags().Lookup("background")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind background flag: %v\n", err)
	}
	
	// Bind pricing and deduplication flags
	if err := viper.BindPFlag("data.pricing_source", rootCmd.Flags().Lookup("pricing-source")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind pricing-source flag: %v\n", err)
	}
	if err := viper.BindPFlag("data.pricing_offline_mode", rootCmd.Flags().Lookup("pricing-offline")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind pricing-offline flag: %v\n", err)
	}
	if err := viper.BindPFlag("data.deduplication", rootCmd.Flags().Lookup("deduplication")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind deduplication flag: %v\n", err)
	}
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".claudecat" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".claudecat")
	}

	// Environment variable prefix
	viper.SetEnvPrefix("CLAWCAT")
	viper.AutomaticEnv()

	// Set default values
	setDefaults()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

func setDefaults() {
	// App defaults
	viper.SetDefault("app.name", "claudecat")
	viper.SetDefault("app.refresh_interval", "1s")
	viper.SetDefault("app.data_paths", []string{})

	// UI defaults
	viper.SetDefault("ui.theme", "dark")
	viper.SetDefault("ui.no_color", false)
	viper.SetDefault("ui.show_help", true)

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.verbose", false)
	viper.SetDefault("log.file", "")

	// Debug defaults
	viper.SetDefault("debug.enabled", false)
	viper.SetDefault("debug.metrics_port", 0)
	viper.SetDefault("debug.pprof_port", 0)

	// Cache defaults
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.max_size", "100MB")

	// Session defaults
	viper.SetDefault("sessions.idle_timeout", "5m")
	viper.SetDefault("sessions.max_sessions", 1000)

	// Export defaults
	viper.SetDefault("export.default_format", "csv")
	viper.SetDefault("export.compress", false)
}

func initializeConfig() error {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(viper.ConfigFileUsed())
	if configDir != "" && configDir != "." {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

func loadConfiguration(cmd *cobra.Command) (*config.Config, error) {
	// Create config loader
	loader := config.NewLoader()

	// Add default configuration paths as file sources
	for _, path := range config.ConfigPaths() {
		loader.AddSource(config.NewFileSource(path))
	}

	// Add environment variable source
	loader.AddSource(config.NewEnvSource("CLAWCAT"))

	// Add command line flags source
	loader.AddSource(config.NewFlagSource(cmd.Flags()))

	// Add validator
	loader.AddValidator(config.NewStandardValidator())

	// Load configuration with defaults as fallback
	cfg, err := loader.LoadWithDefaults()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyRunFlags(cfg *config.Config) error {
	// Apply data paths if provided
	if len(runPaths) > 0 {
		// Validate paths exist
		for _, path := range runPaths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}
		}
		cfg.Data.Paths = runPaths
	}

	// Apply subscription plan if provided
	if runPlan != "" {
		validPlans := []string{"free", "pro", "team"}
		found := false
		for _, plan := range validPlans {
			if strings.EqualFold(runPlan, plan) {
				cfg.Subscription.Plan = strings.ToLower(runPlan)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid subscription plan: %s (valid options: %s)",
				runPlan, strings.Join(validPlans, ", "))
		}
	}

	// Apply refresh interval if provided
	if runRefresh > 0 {
		if runRefresh < 100*time.Millisecond {
			return fmt.Errorf("refresh interval too small: %v (minimum: 100ms)", runRefresh)
		}
		if runRefresh > 1*time.Minute {
			return fmt.Errorf("refresh interval too large: %v (maximum: 1m)", runRefresh)
		}
		cfg.UI.RefreshRate = runRefresh
	}

	// Apply theme if provided
	if runTheme != "" {
		validThemes := []string{"dark", "light", "high-contrast"}
		found := false
		for _, theme := range validThemes {
			if strings.EqualFold(runTheme, theme) {
				cfg.UI.Theme = strings.ToLower(runTheme)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid theme: %s (valid options: %s)",
				runTheme, strings.Join(validThemes, ", "))
		}
	}

	// Apply watch flag
	if runWatch {
		cfg.Data.AutoDiscover = true
	}

	// Apply background mode
	if runBackground {
		cfg.UI.CompactMode = true
	}

	// Apply pricing source if provided
	if pricingSource != "" {
		validSources := []string{"default", "litellm"}
		found := false
		for _, source := range validSources {
			if strings.EqualFold(pricingSource, source) {
				cfg.Data.PricingSource = strings.ToLower(pricingSource)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid pricing source: %s (valid options: %s)",
				pricingSource, strings.Join(validSources, ", "))
		}
	}

	// Apply pricing offline mode if set
	if pricingOffline {
		cfg.Data.PricingOfflineMode = true
	}

	// Apply deduplication if set
	if enableDeduplication {
		cfg.Data.Deduplication = true
	}

	return nil
}
