package fileio

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type PathDiscovery struct {
	searchPaths []string
	filters     []FilterFunc
}

type FilterFunc func(path string) bool

type DiscoveredPath struct {
	Path         string
	ProjectCount int
	LastModified time.Time
	Size         int64
}

func NewPathDiscovery() *PathDiscovery {
	pd := &PathDiscovery{
		searchPaths: getDefaultSearchPaths(),
		filters:     []FilterFunc{},
	}

	// Add default filters
	pd.AddFilter(isValidClaudeDirectory)
	pd.AddFilter(hasConversationFiles)

	return pd
}

func (p *PathDiscovery) Discover() ([]DiscoveredPath, error) {
	var discovered []DiscoveredPath

	for _, searchPath := range p.searchPaths {
		paths, err := p.discoverInPath(searchPath)
		if err != nil {
			continue // Skip paths that can't be accessed
		}
		discovered = append(discovered, paths...)
	}

	return discovered, nil
}

func (p *PathDiscovery) AddSearchPath(path string) {
	// Expand user home directory
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[1:])
		}
	}
	p.searchPaths = append(p.searchPaths, path)
}

func (p *PathDiscovery) AddFilter(filter FilterFunc) {
	p.filters = append(p.filters, filter)
}

func (p *PathDiscovery) discoverInPath(searchPath string) ([]DiscoveredPath, error) {
	var discovered []DiscoveredPath

	// Check if the search path exists
	info, err := os.Stat(searchPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", searchPath)
	}

	// Look for project directories within the search path
	entries, err := os.ReadDir(searchPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(searchPath, entry.Name())

		// Apply filters
		if !p.passesFilters(projectPath) {
			continue
		}

		// Get path statistics
		stats, err := getPathStats(projectPath)
		if err != nil {
			continue
		}

		discovered = append(discovered, stats)
	}

	return discovered, nil
}

func (p *PathDiscovery) passesFilters(path string) bool {
	for _, filter := range p.filters {
		if !filter(path) {
			return false
		}
	}
	return true
}

func getDefaultSearchPaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			filepath.Join(homeDir, ".claude", "projects"),
			filepath.Join(homeDir, ".config", "claude", "projects"),
			filepath.Join(homeDir, "Library", "Application Support", "claude", "projects"),
		}
	case "linux":
		paths = []string{
			filepath.Join(homeDir, ".claude", "projects"),
			filepath.Join(homeDir, ".config", "claude", "projects"),
			filepath.Join(homeDir, ".local", "share", "claude", "projects"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		localAppData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		paths = []string{
			filepath.Join(appData, "claude", "projects"),
			filepath.Join(localAppData, "claude", "projects"),
		}
	default:
		// Fallback for unknown systems
		paths = []string{
			filepath.Join(homeDir, ".claude", "projects"),
			filepath.Join(homeDir, ".config", "claude", "projects"),
		}
	}

	return paths
}

func isValidClaudeDirectory(path string) bool {
	// Check if the directory looks like a Claude project directory
	// Look for common patterns: UUID-like names or conversation files
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	// If empty, it's not a valid project directory
	if len(entries) == 0 {
		return false
	}

	// Look for files that indicate this is a Claude project
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Look for conversation files (typically have UUID-like names)
		if strings.Contains(name, "-") && len(name) > 30 {
			return true
		}
		// Look for JSONL files
		if strings.HasSuffix(name, ".jsonl") {
			return true
		}
	}

	return false
}

func hasConversationFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	conversationCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Count files that look like conversation files
		if strings.HasSuffix(name, ".jsonl") ||
			(strings.Contains(name, "-") && len(name) > 20) {
			conversationCount++
		}
	}

	// Require at least one conversation file
	return conversationCount > 0
}

func getPathStats(path string) (DiscoveredPath, error) {
	info, err := os.Stat(path)
	if err != nil {
		return DiscoveredPath{}, err
	}

	// Count conversation files
	projectCount := 0
	totalSize := int64(0)
	lastModified := info.ModTime()

	err = filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if fileInfo.IsDir() {
			return nil
		}

		name := fileInfo.Name()
		if strings.HasSuffix(name, ".jsonl") ||
			(strings.Contains(name, "-") && len(name) > 20) {
			projectCount++
			totalSize += fileInfo.Size()

			if fileInfo.ModTime().After(lastModified) {
				lastModified = fileInfo.ModTime()
			}
		}

		return nil
	})

	if err != nil {
		return DiscoveredPath{}, err
	}

	return DiscoveredPath{
		Path:         path,
		ProjectCount: projectCount,
		LastModified: lastModified,
		Size:         totalSize,
	}, nil
}

// DiscoverDataPaths is a convenience function to discover all Claude data paths
// DiscoverFiles discovers JSONL files in a given path
func DiscoverFiles(path string) ([]string, error) {
	var files []string
	
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	
	if info.IsDir() {
		// Search for JSONL files in directory
		err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(walkPath), ".jsonl") {
				files = append(files, walkPath)
			}
			
			return nil
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Single file
		if strings.HasSuffix(strings.ToLower(path), ".jsonl") {
			files = append(files, path)
		}
	}
	
	return files, nil
}

// GetDefaultPaths returns the default paths for Claude data
func GetDefaultPaths() ([]string, error) {
	return DiscoverDataPaths()
}

func DiscoverDataPaths() ([]string, error) {
	discovery := NewPathDiscovery()
	discovered, err := discovery.Discover()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, d := range discovered {
		paths = append(paths, d.Path)
	}

	return paths, nil
}
