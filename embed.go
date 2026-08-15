// Package winforge embeds the static web UI and default configuration into the
// final binary so a single .exe is fully self-contained.
package winforge

import "embed"

// Assets holds the embedded web dashboard and default config files.
//
//go:embed web config
var Assets embed.FS
