// Package web contiene le pagine servite dal servizio, incluse nell'eseguibile:
//
//   - la dashboard (index.html, dashboard.css, dashboard.js), servita su /app/
//   - public/, copia della web app di ../docs fatta da `make assets`, servita su
//     /crea/: genera QR statici nel browser, senza accesso
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html dashboard.css dashboard.js public
var files embed.FS

// App è la dashboard.
func App() fs.FS {
	sub, _ := fs.Sub(files, ".")
	return sub
}

// Public è il generatore di QR statici (la web app di docs/).
func Public() fs.FS {
	sub, err := fs.Sub(files, "public")
	if err != nil {
		panic(err)
	}
	return sub
}
