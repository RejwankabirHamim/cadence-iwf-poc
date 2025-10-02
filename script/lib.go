package script

import "embed"

//go:embed *.tmpl
//go:embed **/*.tmpl
//go:embed **/*.sh
//go:embed **/**/*.tmpl
//go:embed **/**/**/*.tmpl
var FS embed.FS
