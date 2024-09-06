package itermultipart_test

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/textproto"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xakep666/itermultipart"
)

func TestNameAccessors(t *testing.T) {
	t.Run("general", func(t *testing.T) {
		tests := [...][3]string{
			{`form-data; name="foo"`, "foo", ""},
			{` form-data ; name=foo`, "foo", ""},
			{`FORM-DATA;name="foo"`, "foo", ""},
			{` FORM-DATA ; name="foo"`, "foo", ""},
			{` FORM-DATA ; name="foo"`, "foo", ""},
			{` FORM-DATA ; name=foo`, "foo", ""},
			{` FORM-DATA ; filename="foo.txt"; name=foo; baz=quux`, "foo", "foo.txt"},
			{` not-form-data ; filename="bar.txt"; name=foo; baz=quux`, "", "bar.txt"},
		}
		for i, test := range tests {
			p := &itermultipart.Part{Header: make(textproto.MIMEHeader)}
			p.Header.Set("Content-Disposition", test[0])
			if g, e := p.FormName(), test[1]; g != e {
				t.Errorf("test %d: FormName() = %q; want %q", i, g, e)
			}
			if g, e := p.FileName(), test[2]; g != e {
				t.Errorf("test %d: FileName() = %q; want %q", i, g, e)
			}
		}
	})

	t.Run("disposition changed between calls", func(t *testing.T) {
		p := &itermultipart.Part{Header: make(textproto.MIMEHeader)}
		p.Header.Set("Content-Disposition", `form-data; name="foo"`)
		if g, e := p.FormName(), "foo"; g != e {
			t.Errorf("FormName() = %q; want %q", g, e)
		}
		if g, e := p.FileName(), ""; g != e {
			t.Errorf("FileName() = %q; want %q", g, e)
		}

		p.Header.Set("Content-Disposition", `form-data; name="bar"; filename="baz.txt"`)
		if g, e := p.FormName(), "bar"; g != e {
			t.Errorf("FormName() = %q; want %q", g, e)
		}
		if g, e := p.FileName(), "baz.txt"; g != e {
			t.Errorf("FileName() = %q; want %q", g, e)
		}
	})
}

func ExampleNewPart() {
	part := itermultipart.NewPart().
		SetFormName("customfile").
		SetFileName("example.txt").
		SetContentTypeByExtension().
		SetHeaderValue("X-Custom-Header", "value").
		SetContentString("Hello, World!")

	for _, k := range slices.Sorted(maps.Keys(part.Header)) {
		fmt.Printf("%s: %s\n", k, part.Header[k])
	}
	fmt.Println("---")
	io.Copy(os.Stdout, part.Content)
	// Output:
	// Content-Disposition: [form-data; filename=example.txt; name=customfile]
	// Content-Type: [text/plain; charset=utf-8]
	// X-Custom-Header: [value]
	// ---
	// Hello, World!
}

func ExamplePart_AddToWriter() {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.SetBoundary("boundary")

	itermultipart.NewPart().
		SetFormName("customfile").
		SetFileName("example.txt").
		SetContentTypeByExtension().
		SetHeaderValue("X-Custom-Header", "value").
		SetContentString("Hello, World!").
		AddToWriter(mw)

	itermultipart.NewPart().
		SetFormName("key").
		SetContentString("val").
		AddToWriter(mw)

	mw.Close()

	fmt.Println(strings.ReplaceAll(buf.String(), "\r\n", "\n"))
	// Output:
	// --boundary
	// Content-Disposition: form-data; filename=example.txt; name=customfile
	// Content-Type: text/plain; charset=utf-8
	// X-Custom-Header: value
	//
	// Hello, World!
	// --boundary
	// Content-Disposition: form-data; name=key
	//
	// val
	// --boundary--
}

func ExamplePart_DetectContentType() {
	part := itermultipart.NewPart().
		SetFormName("customfile").
		SetFileName("example.txt").
		SetContentString("<html><body>test</body></html>").
		DetectContentType()

	fmt.Println(part.ContentType())
	// Output:
	// text/html; charset=utf-8
}
