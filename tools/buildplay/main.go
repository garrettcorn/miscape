// Command buildplay assembles the in-browser world into site/play/:
// the game client (web/), the engine compiled to WebAssembly, and Go's
// wasm_exec.js loader. Used by the Pages workflow and for local previews:
//
//	go run ./tools/buildplay            (then: go run ./tools/sitepreview)
//
// LF_VERSION sets the version shown in-game (defaults to "dev").
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const out = "site/play"

func main() {
	log.SetFlags(0)
	if err := os.RemoveAll(out); err != nil {
		log.Fatal(err)
	}
	must(os.MkdirAll(filepath.Join(out, "js"), 0o755))

	version := os.Getenv("LF_VERSION")
	if version == "" {
		version = "dev"
	}
	cmd := exec.Command("go", "build", "-trimpath",
		"-ldflags", "-s -w -X lanternfall/internal/server.Version="+version,
		"-o", filepath.Join(out, "lanternfall.wasm"), "./cmd/lanternfall-web")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	must(cmd.Run())

	// Go's loader script ships with the toolchain (lib/wasm since Go 1.24).
	root := strings.TrimSpace(run("go", "env", "GOROOT"))
	execJS := filepath.Join(root, "lib", "wasm", "wasm_exec.js")
	if _, err := os.Stat(execJS); err != nil {
		execJS = filepath.Join(root, "misc", "wasm", "wasm_exec.js")
	}
	copyFile(execJS, filepath.Join(out, "wasm_exec.js"))

	copyFile("web/style.css", filepath.Join(out, "style.css"))
	ents, err := os.ReadDir("web/js")
	must(err)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".js") {
			copyFile(filepath.Join("web/js", e.Name()), filepath.Join(out, "js", e.Name()))
		}
	}
	// Mark the page as a browser world so the client starts the worker.
	html, err := os.ReadFile("web/index.html")
	must(err)
	marker := []byte(`<meta charset="utf-8">`)
	if !bytes.Contains(html, marker) {
		log.Fatal("web/index.html: charset meta not found")
	}
	html = bytes.Replace(html, marker, append(marker, []byte("\n<meta name=\"lanternfall-mode\" content=\"browser\">")...), 1)
	html = bytes.Replace(html, []byte("<title>Lanternfall</title>"), []byte("<title>Lanternfall — play in your browser</title>"), 1)
	must(os.WriteFile(filepath.Join(out, "index.html"), html, 0o644))

	st, _ := os.Stat(filepath.Join(out, "lanternfall.wasm"))
	fmt.Printf("built %s (engine %.1f MB, %s/%s host)\n", out, float64(st.Size())/1e6, runtime.GOOS, runtime.GOARCH)
}

func run(name string, args ...string) string {
	b, err := exec.Command(name, args...).Output()
	must(err)
	return string(b)
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	must(err)
	defer in.Close()
	o, err := os.Create(dst)
	must(err)
	defer o.Close()
	_, err = io.Copy(o, in)
	must(err)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
