//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestBuildPermisosNuevo comprueba que un .docx nuevo recibe 0o666 filtrado por
// la umask del proceso (como os.Create), y no los 0600 del temporal.
func TestBuildPermisosNuevo(t *testing.T) {
	vieja := syscall.Umask(0o022)
	defer syscall.Umask(vieja)

	in := escribir(t, "in.md", "# hola")
	out := filepath.Join(t.TempDir(), "o.docx")
	if code, _, se := exec(t, "", "build", in, "-o", out); code != 0 {
		t.Fatalf("code=%d %s", code, se)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Mode().Perm(); got != 0o644 {
		t.Errorf("permisos del archivo nuevo = %o, esperaba 0644", got)
	}
}

// TestBuildConservaPermisos comprueba que si la salida ya existía, se conservan
// sus permisos en lugar de imponer los del archivo nuevo.
func TestBuildConservaPermisos(t *testing.T) {
	vieja := syscall.Umask(0o022)
	defer syscall.Umask(vieja)

	in := escribir(t, "in.md", "# hola")
	out := filepath.Join(t.TempDir(), "o.docx")
	if err := os.WriteFile(out, []byte("viejo"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(out, 0o640); err != nil {
		t.Fatal(err)
	}
	if code, _, se := exec(t, "", "build", in, "-o", out); code != 0 {
		t.Fatalf("code=%d %s", code, se)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Mode().Perm(); got != 0o640 {
		t.Errorf("permisos conservados = %o, esperaba 0640", got)
	}
}
