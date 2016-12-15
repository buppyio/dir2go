package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"
)

func main() {
	dir := flag.String("dir", "", "The path of the directory to embed")
	pkgName := flag.String("pkgname", "", "The name of the package to generate")
	flag.Parse()

	if *dir == "" {
		fmt.Fprintf(os.Stderr, "missing -dir\n")
		os.Exit(1)
	}

	if *pkgName == "" {
		fmt.Fprintf(os.Stderr, "missing -pkgname\n")
		os.Exit(1)
	}

	archiveBytes, err := getArchiveBytes(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating package %s from %s: %s\n", *pkgName, *dir, err)
		os.Exit(1)
	}

	err = writePackage(os.Stdout, *pkgName, archiveBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing package %s\n", *pkgName, err)
		os.Exit(1)
	}
}

func getArchiveBytes(dir string) ([]byte, error) {
	buf := bytes.Buffer{}

	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	tw := tar.NewWriter(w)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = path[len(dir):]

		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking input directive: %s", err)
	}

	err = tw.Close()
	if err != nil {
		return nil, fmt.Errorf("error creating archive: %s", err)
	}

	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("closing gzip stream: %s", err)
	}

	return buf.Bytes(), nil
}

func bytesToGoString(data []byte) string {
	buf := bytes.Buffer{}
	for _, b := range data {
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

import (
	"io/ioutil"
	"compress/gzip"
)

type EmbeddedFile struct {
	Info os.FileInfo
	Contents []byte
}

var files map[string]*EmbeddedFile
var initialized = false

func GetFiles() map[string]*EmbeddedFile {
	if !initialized {
		loadFiles()
		initialized = true
	}
	return files
}

func loadFiles() {
	files = make(map[string]*EmbeddedFile)

	tr, err := tar.NewReader(gzip.NewReader(bytes.NewBuffer(data)))
	if err != nil {
		panic(err)
	}

	files = make(map[string]*EmbeddedFile)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		if hdr.TypeFlag != tar.TypeReg {
			continue
		}

		contents, err := ioutil.ReadFull(tr)
		if err != nil {
			panic(err)
		}
	}

}


var data string = {{.DataString}}
`

func init() {
	packageTemplate = template.Must(template.New("package").Parse(packageTemplateText))
}
