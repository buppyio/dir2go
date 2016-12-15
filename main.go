package main

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"
)

func main() {
	srcDir := flag.String("srcdir", "", "The path of the directory to embed")
	pkgName := flag.String("pkgname", "", "The name of the package to generate")
	flag.Parse()

	if *srcDir == "" {
		fmt.Fprintf(os.Stderr, "missing -srcdir\n")
		os.Exit(1)
	}

	if *pkgName == "" {
		fmt.Fprintf(os.Stderr, "missing -pkgname\n")
		os.Exit(1)
	}

	archiveBytes, err := getArchiveBytes(*srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating package %s from %s: %s\n", *pkgName, *srcDir, err)
		os.Exit(1)
	}

	err = writePackage(os.Stdout, *pkgName, archiveBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing package %s\n", *pkgName, err)
		os.Exit(1)
	}
}

func getArchiveBytes(srcDir string) ([]byte, error) {
	buf := bytes.Buffer{}
	w := tar.NewWriter(&buf)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking input directive: %s", err)
	}

	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("error creating archive: %s", err)
	}

	return buf.Bytes(), nil
}

func bytesToGoString(data []byte) string {
	buf := bytes.Buffer{}
	for _, b := range data {
		if b == '\n' {
			buf.WriteString(`\n`)
			continue
		}
		if b == '\\' {
			buf.WriteString(`\\`)
			continue
		}
		if b == '"' {
			buf.WriteString(`\"`)
			continue
		}
		if (b >= 32 && b <= 126) || b == '\t' {
			buf.WriteByte(b)
			continue
		}
		_, _ = fmt.Fprintf(&buf, "\\x%02x", b)
	}
	return `"` + buf.String() + `"`
}

func writePackage(w io.Writer, pkgName string, archive []byte) error {
	err := packageTemplate.Execute(w, struct {
		PkgName    string
		DataString string
	}{
		PkgName:    pkgName,
		DataString: bytesToGoString(archive),
	})
	return err
}

var packageTemplate *template.Template
var packageTemplateText string = `
package {{.PkgName}}

var data string = {{.DataString}}
`

func init() {
	packageTemplate = template.Must(template.New("package").Parse(packageTemplateText))
}
