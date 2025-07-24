package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	logLevel  string
	noColor   bool
	debug     bool
	verbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "clawcat",
	Short: "Claude Code Usage Monitor",
	Long: `ClawCat is a high-performance TUI application for monitoring Claude AI token usage and costs.

It provides real-time monitoring, session analysis, cost calculations, and data export
capabilities to help developers track their Claude API usage efficiently.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig()
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
			cmd.Printf("Error: unknown command \"help\" for \"clawcat\"\n")
			cmd.Printf("Run 'clawcat --help' for usage.\n")
		},
	})

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.clawcat.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("ui.no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("debug.enabled", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("log.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
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

		// Search config in home directory with name ".clawcat" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".clawcat")
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
	viper.SetDefault("app.name", "ClawCat")
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
	viper.SetDefault("cache.ttl", "1h")

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