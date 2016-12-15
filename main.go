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
		if !info.Mode().IsRegular() {
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
var packageTemplateText string = `package {{.PkgName}}

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

type EmbeddedFile struct {
	Info     os.FileInfo
	Contents []byte
}

type EmbeddedFileHandle struct {
	File *EmbeddedFile
	R    *bytes.Reader
}

func (h *EmbeddedFileHandle) Read(buf []byte) (int, error) {
	return h.R.Read(buf)
}

func (h *EmbeddedFileHandle) Seek(offset int64, whence int) (int64, error) {
	return h.R.Seek(offset, whence)
}

func (h *EmbeddedFileHandle) Close() error {
	return nil
}

func (h *EmbeddedFileHandle) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrPermission
}

func (h *EmbeddedFileHandle) Stat() (os.FileInfo, error) {
	return h.File.Info, nil
}

type FileSystem map[string]*EmbeddedFile

func (fs FileSystem) Open(name string) (http.File, error) {
	f, ok := fs[name[1:]]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &EmbeddedFileHandle{
		File: f,
		R:    bytes.NewReader(f.Contents),
	}, nil
}

func LoadFiles() {
	Files = make(FileSystem)

	gz, err := gzip.NewReader(bytes.NewBuffer([]byte(data)))
	if err != nil {
		panic(err)
	}

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		contents, err := ioutil.ReadAll(tr)
		if err != nil {
			panic(err)
		}

		Files[hdr.Name] = &EmbeddedFile{
			Info:     hdr.FileInfo(),
			Contents: contents,
		}
	}

}

var Files FileSystem
var data string = {{.DataString}}
`

func init() {
	packageTemplate = template.Must(template.New("package").Parse(packageTemplateText))
}
