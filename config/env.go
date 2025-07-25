package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// EnvMapper handles mapping environment variables to configuration fields
type EnvMapper struct {
	prefix   string
	mappings map[string]string
}

// NewEnvMapper creates a new environment variable mapper
func NewEnvMapper(prefix string) *EnvMapper {
	return &EnvMapper{
		prefix:   prefix,
		mappings: make(map[string]string),
	}
}

// Map adds a mapping from environment variable key to configuration path
func (e *EnvMapper) Map(envKey, configPath string) {
	e.mappings[envKey] = configPath
}

// Apply applies environment variable mappings to configuration
func (e *EnvMapper) Apply(cfg *Config) error {
	// Apply standard mappings first
	for envKey, configPath := range StandardEnvMappings {
		if value := os.Getenv(envKey); value != "" {
			if err := e.setFieldByPath(cfg, configPath, value); err != nil {
				return fmt.Errorf("failed to set %s from %s: %w", configPath, envKey, err)
			}
		}
	}

	// Apply custom mappings
	for envKey, configPath := range e.mappings {
		fullEnvKey := e.prefix + "_" + envKey
		if value := os.Getenv(fullEnvKey); value != "" {
			if err := e.setFieldByPath(cfg, configPath, value); err != nil {
				return fmt.Errorf("failed to set %s from %s: %w", configPath, fullEnvKey, err)
			}
		}
	}

	return nil
}

// setFieldByPath sets a configuration field by its dot-separated path
func (e *EnvMapper) setFieldByPath(cfg *Config, path, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid path: %s", path)
	}

	// Get the struct field
	v := reflect.ValueOf(cfg).Elem()
	for i, part := range parts[:len(parts)-1] {
		field := v.FieldByName(e.toCamelCase(part))
		if !field.IsValid() {
			return fmt.Errorf("invalid field path at %s: %s", strings.Join(parts[:i+1], "."), part)
		}
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}
		v = field
	}

	// Set the final field
	fieldName := e.toCamelCase(parts[len(parts)-1])
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("invalid field: %s", fieldName)
	}
	if !field.CanSet() {
		return fmt.Errorf("cannot set field: %s", fieldName)
	}

	return e.setFieldValue(field, value)
}

// setFieldValue sets a field value based on its type
func (e *EnvMapper) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool value: %s", value)
		}
		field.SetBool(boolVal)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration value: %s", value)
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid int value: %s", value)
			}
			if field.OverflowInt(intVal) {
				return fmt.Errorf("int value overflow: %s", value)
			}
			field.SetInt(intVal)
		}

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %s", value)
		}
		if field.OverflowFloat(floatVal) {
			return fmt.Errorf("float value overflow: %s", value)
		}
		field.SetFloat(floatVal)

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			// Handle string slices (comma-separated values)
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		} else {
			return fmt.Errorf("unsupported slice type: %s", field.Type())
		}

	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}

	return nil
}

// toCamelCase converts snake_case to CamelCase with proper casing
func (e *EnvMapper) toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			// Handle common abbreviations
			switch strings.ToLower(part) {
			case "ui":
				parts[i] = "UI"
			case "gc":
				parts[i] = "GC"
			case "cpu":
				parts[i] = "CPU"
			case "api":
				parts[i] = "API"
			case "url":
				parts[i] = "URL"
			case "id":
				parts[i] = "ID"
			default:
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
	}
	return strings.Join(parts, "")
}

// StandardEnvMappings defines the standard environment variable mappings
var StandardEnvMappings = map[string]string{
	// App configuration
	"CLAWCAT_LOG_LEVEL": "app.log_level",
	"CLAWCAT_LOG_FILE":  "app.log_file",
	"CLAWCAT_TIMEZONE":  "app.timezone",

	// Data configuration
	"CLAWCAT_DATA_PATHS":     "data.paths",
	"CLAWCAT_AUTO_DISCOVER":  "data.auto_discover",
	"CLAWCAT_WATCH_INTERVAL": "data.watch_interval",
	"CLAWCAT_MAX_FILE_SIZE":  "data.max_file_size",
	"CLAWCAT_CACHE_ENABLED":  "data.cache_enabled",
	"CLAWCAT_CACHE_SIZE":     "data.cache_size",

	// UI configuration
	"CLAWCAT_THEME":           "ui.theme",
	"CLAWCAT_REFRESH_RATE":    "ui.refresh_rate",
	"CLAWCAT_COMPACT_MODE":    "ui.compact_mode",
	"CLAWCAT_SHOW_SPINNER":    "ui.show_spinner",
	"CLAWCAT_CHART_HEIGHT":    "ui.chart_height",
	"CLAWCAT_TABLE_PAGE_SIZE": "ui.table_page_size",
	"CLAWCAT_DATE_FORMAT":     "ui.date_format",
	"CLAWCAT_TIME_FORMAT":     "ui.time_format",

	// Performance configuration
	"CLAWCAT_WORKER_COUNT": "performance.worker_count",
	"CLAWCAT_BUFFER_SIZE":  "performance.buffer_size",
	"CLAWCAT_BATCH_SIZE":   "performance.batch_size",
	"CLAWCAT_MAX_MEMORY":   "performance.max_memory",
	"CLAWCAT_GC_INTERVAL":  "performance.gc_interval",

	// Subscription configuration
	"CLAWCAT_PLAN":               "subscription.plan",
	"CLAWCAT_CUSTOM_TOKEN_LIMIT": "subscription.custom_token_limit",
	"CLAWCAT_CUSTOM_COST_LIMIT":  "subscription.custom_cost_limit",
	"CLAWCAT_WARN_THRESHOLD":     "subscription.warn_threshold",
	"CLAWCAT_ALERT_THRESHOLD":    "subscription.alert_threshold",

	// Debug configuration
	"CLAWCAT_DEBUG":          "debug.enabled",
	"CLAWCAT_PROFILE_CPU":    "debug.profile_cpu",
	"CLAWCAT_PROFILE_MEMORY": "debug.profile_memory",
	"CLAWCAT_TRACE_FILE":     "debug.trace_file",
	"CLAWCAT_METRICS_PORT":   "debug.metrics_port",
}

// GetEnvWithDefault gets an environment variable with a default value
func GetEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvBool gets an environment variable as boolean
func GetEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// GetEnvInt gets an environment variable as integer
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetEnvFloat gets an environment variable as float64
func GetEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

// GetEnvDuration gets an environment variable as time.Duration
func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetEnvSlice gets an environment variable as string slice (comma-separated)
func GetEnvSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = strings.TrimSpace(part)
	}
	return result
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv(cfg *Config) error {
	mapper := NewEnvMapper("CLAWCAT")
	return mapper.Apply(cfg)
}

// SetEnvDefaults sets default environment variables for configuration
func SetEnvDefaults() {
	envDefaults := map[string]string{
		"CLAWCAT_LOG_LEVEL":    "info",
		"CLAWCAT_THEME":        "dark",
		"CLAWCAT_PLAN":         "pro",
		"CLAWCAT_CACHE_SIZE":   "50",
		"CLAWCAT_WORKER_COUNT": strconv.Itoa(4),
	}

	for key, value := range envDefaults {
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
