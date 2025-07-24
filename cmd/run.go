package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/internal"
)

var (
	runPaths        []string
	runPlan         string
	runRefresh      time.Duration
	runTheme        string
	runWatch        bool
	runBackground   bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the interactive TUI monitor",
	Long: `Start ClawCat in interactive TUI mode with real-time monitoring.

This command launches the main Terminal User Interface (TUI) that provides:
- Real-time token usage monitoring
- Session analysis and tracking
- Cost calculations and budgeting
- Multiple view modes (dashboard, sessions, analytics)
- File watching for automatic updates

Examples:
  clawcat run                                    # Run with default settings
  clawcat run --paths ~/claude-logs             # Monitor specific directory
  clawcat run --refresh 5s --theme light        # Custom refresh rate and theme
  clawcat run --watch --background              # Watch files in background`,
	
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load and validate configuration
		cfg, err := loadConfiguration()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Override configuration with command line flags
		if err := applyRunFlags(cfg); err != nil {
			return fmt.Errorf("failed to apply command flags: %w", err)
		}

		// Configuration is validated by the validator in the config package

		// Create and run application
		app, err := internal.NewApplication(cfg)
		if err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Starting ClawCat TUI monitor...\n")
			fmt.Fprintf(os.Stderr, "Configuration: %+v\n", cfg)
		}

		return app.Run()
	},
}

func init() {
	// Command-specific flags
	runCmd.Flags().StringSliceVarP(&runPaths, "paths", "p", nil, "data paths to monitor (can be specified multiple times)")
	runCmd.Flags().StringVar(&runPlan, "plan", "", "subscription plan (free, pro, team)")
	runCmd.Flags().DurationVarP(&runRefresh, "refresh", "r", 0, "refresh interval (e.g., 1s, 500ms)")
	runCmd.Flags().StringVarP(&runTheme, "theme", "t", "", "UI theme (dark, light, high-contrast)")
	runCmd.Flags().BoolVarP(&runWatch, "watch", "w", false, "enable file watching for real-time updates")
	runCmd.Flags().BoolVar(&runBackground, "background", false, "run in background mode (minimal UI)")

	// Bind flags to viper for configuration
	viper.BindPFlag("app.data_paths", runCmd.Flags().Lookup("paths"))
	viper.BindPFlag("app.subscription_plan", runCmd.Flags().Lookup("plan"))
	viper.BindPFlag("app.refresh_interval", runCmd.Flags().Lookup("refresh"))
	viper.BindPFlag("ui.theme", runCmd.Flags().Lookup("theme"))
	viper.BindPFlag("fileio.watch_enabled", runCmd.Flags().Lookup("watch"))
	viper.BindPFlag("app.background_mode", runCmd.Flags().Lookup("background"))

	// Add to root command
	rootCmd.AddCommand(runCmd)
}

func loadConfiguration() (*config.Config, error) {
	// Create config loader
	loader := config.NewLoader()

	// Load configuration from multiple sources
	cfg, err := loader.Load()
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

	return nil
}