package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// Static 返回前端静态资源文件系统。
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
