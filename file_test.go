package livepkg

import (
	"bytes"
	"testing"
)

func TestSourceReadJS(t *testing.T) {
	src := &Source{Ext: ".js"}
	src.ReadFrom(bytes.NewBufferString(`
		depends("/A");
		depends('/B')
	`))

	if !sameDeps(src.Deps, []string{"/A", "/B"}) {
		t.Errorf("got %v", src.Deps)
	}
}

func TestSourceReadCSS(t *testing.T) {
	src := &Source{Ext: ".css"}
	src.ReadFrom(bytes.NewBufferString(`
		@depends   "/A";
		@depends	"/B" ;
	`))

	if !sameDeps(src.Deps, []string{"/A", "/B"}) {
		t.Errorf("got %v", src.Deps)
	}
}

func TestSourcePathSanitization(t *testing.T) {
	src := &Source{Path: "/<script>=</script>.js", Ext: ".js"}
	if src.Tag() != `<script src="/%3Cscript%3E=%3C/script%3E.js" type="text/javascript" >` {
		t.Errorf("got %v", src.Tag())
	}
}
