package ui

import "errors"

// Common UI errors
var (
	ErrNoManager         = errors.New("no sessions manager provided")
	ErrNoWatcher         = errors.New("no file watcher provided")
	ErrInvalidParameters = errors.New("invalid parameters provided")
	ErrNoData            = errors.New("no data available")
	ErrUnsupportedFormat = errors.New("unsupported export format")
	ErrViewNotFound      = errors.New("view not found")
	ErrInvalidView       = errors.New("invalid view type")
	ErrComponentNotReady = errors.New("component not ready")
	ErrInvalidDimensions = errors.New("invalid dimensions")
	ErrRenderFailed      = errors.New("render failed")
)
