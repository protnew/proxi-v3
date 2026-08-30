package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDBPath_envWins(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join("C:", "tmp", "custom.db"))
	t.Setenv("DATA_DIR", filepath.Join("C:", "tmp", "datax"))
	db, data := resolveDBPath()
	wantDB := filepath.Join("C:", "tmp", "custom.db")
	wantData := filepath.Join("C:", "tmp", "datax")
	if db != wantDB {
		t.Fatalf("db=%q want=%q", db, wantDB)
	}
	if data != wantData {
		t.Fatalf("data=%q want=%q", data, wantData)
	}
}

func TestResolveDBPath_defaultUnderGoMod(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("DATA_DIR", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "cmd", "webserver")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	db, data := resolveDBPath()
	wantData := filepath.Join(root, "data")
	wantDB := filepath.Join(wantData, "messenger.db")
	if data != wantData {
		t.Fatalf("data=%q want=%q", data, wantData)
	}
	if db != wantDB {
		t.Fatalf("db=%q want=%q", db, wantDB)
	}
}

func TestResolveDBPath_cwdDoesNotCreateSiblingDB(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("DATA_DIR", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "alpha")
	b := filepath.Join(root, "beta")
	if err := os.MkdirAll(a, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(a); err != nil {
		t.Fatal(err)
	}
	_, dataA := resolveDBPath()
	if err := os.Chdir(b); err != nil {
		t.Fatal(err)
	}
	_, dataB := resolveDBPath()
	if dataA != dataB {
		t.Fatalf("cwd changed data dir %q vs %q", dataA, dataB)
	}
	if dataA != filepath.Join(root, "data") {
		t.Fatalf("data=%q", dataA)
	}
}
