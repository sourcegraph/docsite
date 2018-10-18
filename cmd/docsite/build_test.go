package main

import (
	"debug/elf"
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"
)

func TestReadELF(t *testing.T) {
	if err := do(); err != nil {
		t.Fatal(err)
	}
}

func do() error {
	prog, err := exec.LookPath("docsite")
	if err != nil {
		return err
	}
	if out, err := exec.Command("objcopy", "--add-section", "sname=/tmp/foo", prog, prog+".2").CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}

	f, err := os.Open(prog + ".2")
	if err != nil {
		return err
	}
	defer f.Close()
	elfFile, err := elf.NewFile(f)
	if err != nil {
		return err
	}
	for _, section := range elfFile.Sections {
		log.Printf("section %q type %q", section.Name, section.Type)
	}
	return nil
}
