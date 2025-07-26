package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/bytedance/sonic"
	"github.com/spf13/cobra"
)

var (
	versionOutput string
	versionShort  bool
)

// Version information set by linker during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Compiler  string `json:"compiler"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for ClawCat including build details and system information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		versionInfo := VersionInfo{
			Version:   Version,
			BuildTime: BuildTime,
			GitCommit: GitCommit,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			Compiler:  runtime.Compiler,
		}

		switch versionOutput {
		case "json":
			return outputVersionJSON(versionInfo)
		case "short":
			return outputVersionShort(versionInfo)
		default:
			return outputVersionDefault(versionInfo)
		}
	},
}

func init() {
	versionCmd.Flags().StringVarP(&versionOutput, "output", "o", "default", "output format (default, json, short)")
	versionCmd.Flags().BoolVarP(&versionShort, "short", "s", false, "show only version number")

	rootCmd.AddCommand(versionCmd)
}

func outputVersionDefault(info VersionInfo) error {
	fmt.Printf("ClawCat - Claude Code Usage Monitor\n")
	fmt.Printf("Version:     %s\n", info.Version)
	if info.GitCommit != "unknown" {
		fmt.Printf("Git Commit:  %s\n", info.GitCommit)
	}
	if info.BuildTime != "unknown" {
		fmt.Printf("Build Time:  %s\n", info.BuildTime)
	}
	fmt.Printf("Go Version:  %s\n", info.GoVersion)
	fmt.Printf("OS/Arch:     %s/%s\n", info.OS, info.Arch)
	fmt.Printf("Compiler:    %s\n", info.Compiler)
	return nil
}

func outputVersionJSON(info VersionInfo) error {
	data, err := sonic.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte("\n"))
	return err
}

func outputVersionShort(info VersionInfo) error {
	fmt.Println(info.Version)
	return nil
}
