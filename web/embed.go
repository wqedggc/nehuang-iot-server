package web

import "embed"

// Static embeds all frontend static files (admin, mini, mock, version.js)
// into the Go binary so that a single binary can serve the complete app.
//
//go:embed all:static
var Static embed.FS
