package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// parse permite flags antes o después del argumento posicional y devuelve éste.
func parse(fs *flag.FlagSet, args []string) (string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return "", userErr("%v", err)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
	if len(pos) > 1 {
		return "", userErr("argumentos de más: %s", strings.Join(pos[1:], " "))
	}
	if len(pos) == 0 {
		return "", nil
	}
	return pos[0], nil
}

func fprintln(w io.Writer, a ...any) { fmt.Fprintln(w, a...) }

// asUserIfInput clasifica como error de usuario los fallos de imagen (rutas/formatos).
func asUserIfInput(err error) error {
	m := err.Error()
	if strings.HasPrefix(m, "imagen ") {
		return userErr("%s", m)
	}
	return err
}
