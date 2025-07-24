# Module: main

## Overview
The main package serves as the application entry point, orchestrating all modules, handling CLI arguments, managing the application lifecycle, and providing the command-line interface using Cobra.

## Package Structure
```
./
├── main.go             # Entry point
├── cmd/
│   ├── root.go         # Root command
│   ├── run.go          # Main run command
│   ├── analyze.go      # Analyze command
│   ├── export.go       # Export command
│   ├── config.go       # Config command
│   └── version.go      # Version command
├── internal/
│   ├── app.go          # Application orchestrator
│   ├── bootstrap.go    # Initialization logic
│   ├── shutdown.go     # Graceful shutdown
│   └── signals.go      # Signal handling
└── *_test.go           # Integration tests
```

## Main Entry Point

```go
// main.go
package main

import (
    "github.com/penwyp/ClawCat/cmd"
    "github.com/penwyp/ClawCat/internal"
    "log"
    "os"
)

func main() {
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
        os.Exit(1)
    }
}
```

## Command Structure

### Root Command
Base command with global flags.

```go
// cmd/root.go
type RootCmd struct {
    configFile   string
    logLevel     string
    noColor      bool
    debug        bool
}

var rootCmd = &cobra.Command{
    Use:   "clawcat",
    Short: "Claude Code Usage Monitor",
    Long:  `ClawCat is a high-performance TUI application for monitoring Claude AI token usage and costs.`,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Initialize logging
        // Load configuration
        // Set up global context
        return nil
    },
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")
    rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level")
    rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
    rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")
}
```

### Run Command
Main interactive TUI mode.

```go
// cmd/run.go
var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Run the interactive TUI monitor",
    Long:  `Start ClawCat in interactive mode with real-time monitoring.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        app, err := internal.NewApplication(config)
        if err != nil {
            return err
        }
        
        return app.Run()
    },
}

func init() {
    runCmd.Flags().StringSlice("paths", nil, "data paths to monitor")
    runCmd.Flags().String("plan", "", "subscription plan")
    runCmd.Flags().Duration("refresh", time.Second, "refresh interval")
    rootCmd.AddCommand(runCmd)
}
```

### Analyze Command
Non-interactive analysis mode.

```go
// cmd/analyze.go
var analyzeCmd = &cobra.Command{
    Use:   "analyze [flags] [path...]",
    Short: "Analyze usage data without TUI",
    Long:  `Perform analysis on Claude usage data and output results.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        analyzer := internal.NewAnalyzer(config)
        results, err := analyzer.Analyze(args)
        if err != nil {
            return err
        }
        
        return outputResults(results, outputFormat)
    },
}

func init() {
    analyzeCmd.Flags().String("output", "table", "output format (table|json|csv)")
    analyzeCmd.Flags().String("from", "", "start date (YYYY-MM-DD)")
    analyzeCmd.Flags().String("to", "", "end date (YYYY-MM-DD)")
    analyzeCmd.Flags().Bool("by-model", false, "group by model")
    analyzeCmd.Flags().Bool("by-day", false, "group by day")
    rootCmd.AddCommand(analyzeCmd)
}
```

### Export Command
Data export functionality.

```go
// cmd/export.go
var exportCmd = &cobra.Command{
    Use:   "export [flags] <output-file>",
    Short: "Export usage data",
    Long:  `Export Claude usage data to various formats.`,
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        exporter := internal.NewExporter(config)
        return exporter.Export(args[0], exportOptions)
    },
}

type ExportOptions struct {
    Format    string
    TimeRange TimeRange
    Aggregate bool
    Compress  bool
}

func init() {
    exportCmd.Flags().String("format", "csv", "export format (csv|json|xlsx)")
    exportCmd.Flags().String("range", "all", "time range (today|week|month|all)")
    exportCmd.Flags().Bool("aggregate", false, "aggregate data by session")
    exportCmd.Flags().Bool("compress", false, "compress output file")
    rootCmd.AddCommand(exportCmd)
}
```

## Application Orchestrator

```go
// internal/app.go
type Application struct {
    config      *config.Config
    manager     *sessions.Manager
    fileWatcher *fileio.Watcher
    ui          *ui.App
    calculator  *calculations.CostCalculator
    
    ctx         context.Context
    cancel      context.CancelFunc
    wg          sync.WaitGroup
    
    metrics     *Metrics
    logger      *Logger
}

func NewApplication(cfg *config.Config) (*Application, error) {
    app := &Application{
        config: cfg,
        logger: NewLogger(cfg.App.LogLevel),
    }
    
    if err := app.bootstrap(); err != nil {
        return nil, err
    }
    
    return app, nil
}

func (a *Application) Run() error {
    // Set up signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    
    // Start all components
    if err := a.start(); err != nil {
        return err
    }
    
    // Wait for shutdown signal
    <-sigCh
    
    // Graceful shutdown
    return a.shutdown()
}
```

## Bootstrap Process

```go
// internal/bootstrap.go
func (a *Application) bootstrap() error {
    // 1. Validate configuration
    if err := a.validateConfig(); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    
    // 2. Initialize data layer
    if err := a.initializeData(); err != nil {
        return fmt.Errorf("failed to initialize data: %w", err)
    }
    
    // 3. Set up calculations
    a.calculator = calculations.NewCostCalculator()
    
    // 4. Initialize session manager
    a.manager = sessions.NewManager()
    
    // 5. Set up file watching
    if err := a.setupFileWatcher(); err != nil {
        return fmt.Errorf("failed to setup file watcher: %w", err)
    }
    
    // 6. Initialize UI
    if err := a.initializeUI(); err != nil {
        return fmt.Errorf("failed to initialize UI: %w", err)
    }
    
    // 7. Set up metrics if enabled
    if a.config.Debug.MetricsPort > 0 {
        a.metrics = NewMetrics(a.config.Debug.MetricsPort)
    }
    
    return nil
}
```

## Graceful Shutdown

```go
// internal/shutdown.go
func (a *Application) shutdown() error {
    a.logger.Info("Initiating graceful shutdown...")
    
    // Create shutdown context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Signal all components to stop
    a.cancel()
    
    // Stop components in reverse order
    shutdownSteps := []struct {
        name string
        fn   func() error
    }{
        {"UI", a.ui.Stop},
        {"File Watcher", a.fileWatcher.Stop},
        {"Session Manager", a.manager.Stop},
        {"Metrics Server", a.stopMetrics},
    }
    
    var errs []error
    for _, step := range shutdownSteps {
        a.logger.Infof("Stopping %s...", step.name)
        if err := step.fn(); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", step.name, err))
        }
    }
    
    // Wait for all goroutines
    done := make(chan struct{})
    go func() {
        a.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        a.logger.Info("Shutdown complete")
    case <-ctx.Done():
        a.logger.Warn("Shutdown timeout exceeded")
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    
    return nil
}
```

## Signal Handling

```go
// internal/signals.go
func (a *Application) handleSignals() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, 
        os.Interrupt, 
        syscall.SIGTERM,
        syscall.SIGHUP,
        syscall.SIGUSR1,
        syscall.SIGUSR2,
    )
    
    for {
        select {
        case sig := <-sigCh:
            switch sig {
            case os.Interrupt, syscall.SIGTERM:
                a.logger.Info("Received shutdown signal")
                return
                
            case syscall.SIGHUP:
                a.logger.Info("Received SIGHUP, reloading configuration")
                if err := a.reloadConfig(); err != nil {
                    a.logger.Errorf("Failed to reload config: %v", err)
                }
                
            case syscall.SIGUSR1:
                a.logger.Info("Received SIGUSR1, forcing data refresh")
                a.forceRefresh()
                
            case syscall.SIGUSR2:
                a.logger.Info("Received SIGUSR2, dumping debug info")
                a.dumpDebugInfo()
            }
            
        case <-a.ctx.Done():
            return
        }
    }
}
```

## Error Handling

```go
type AppError struct {
    Op   string
    Kind ErrorKind
    Err  error
}

type ErrorKind int
const (
    ErrorConfig ErrorKind = iota
    ErrorData
    ErrorUI
    ErrorSystem
)

func (e *AppError) Error() string {
    return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *AppError) Unwrap() error {
    return e.Err
}
```

## Metrics and Monitoring

```go
type Metrics struct {
    // Application metrics
    StartTime       time.Time
    ProcessedFiles  int64
    ProcessedBytes  int64
    ActiveSessions  int
    
    // Performance metrics
    CPUUsage        float64
    MemoryUsage     uint64
    GoroutineCount  int
    
    // Business metrics
    TotalTokens     int64
    TotalCost       float64
    ErrorCount      int64
}

func (a *Application) collectMetrics() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            a.updateMetrics()
            if a.metrics != nil {
                a.metrics.Export()
            }
        case <-a.ctx.Done():
            return
        }
    }
}
```

## Testing

### Integration Tests
```go
func TestApplicationLifecycle(t *testing.T) {
    // Test full application lifecycle
    cfg := config.TestConfig()
    app, err := NewApplication(cfg)
    require.NoError(t, err)
    
    // Start in goroutine
    errCh := make(chan error)
    go func() {
        errCh <- app.Run()
    }()
    
    // Wait for startup
    time.Sleep(time.Second)
    
    // Trigger shutdown
    syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
    
    // Verify clean shutdown
    err = <-errCh
    assert.NoError(t, err)
}
```

## Build and Release

```makefile
# Makefile
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

build:
    go build -ldflags "$(LDFLAGS)" -o clawcat .

release:
    goreleaser release --clean

install:
    go install -ldflags "$(LDFLAGS)" .
```