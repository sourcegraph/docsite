package main

import (
	"archive/zip"
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestMapFromZipArchive(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		f1, err := zw.Create("a/1")
		if err != nil {
			t.Fatal(err)
		}
		f1.Write([]byte("1"))
		f2, err := zw.Create("c/2")
		if err != nil {
			t.Fatal(err)
		}
		f2.Write([]byte("2"))
		zw.Close()

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		m, err := mapFromZipArchive(zr, "a")
		if err != nil {
			t.Fatal(err)
		}
		if want := map[string]string{"/1": "1"}; !reflect.DeepEqual(m, want) {
			t.Errorf("got %+v, want %+v", m, want)
		}
	})

	t.Run("with symlinks", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		fh := &zip.FileHeader{Name: "a/l"}
		fh.SetMode(0755 | os.ModeSymlink)
		f1, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		f1.Write([]byte("../c/target"))
		f2, err := zw.Create("c/target")
		if err != nil {
			t.Fatal(err)
		}
		f2.Write([]byte("x"))
		zw.Close()

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		m, err := mapFromZipArchive(zr, "a")
		if err != nil {
			t.Fatal(err)
		}
		if want := map[string]string{"/l": "x"}; !reflect.DeepEqual(m, want) {
			t.Errorf("got %+v, want %+v", m, want)
		}
	})
}
