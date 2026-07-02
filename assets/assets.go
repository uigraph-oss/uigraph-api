// Package assets embeds static component icon SVG files for bootstrap upload to object storage.
package assets

import "embed"

//go:embed component-icons/focal/*.svg component-icons/flow/*.svg
var ComponentIcons embed.FS
