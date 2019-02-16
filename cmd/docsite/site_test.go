package main

import (
	"archive/zip"
	"bytes"
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
}
