package itermultipart_test

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/xakep666/itermultipart"
)

func TestSource(t *testing.T) {
	fileContents := []byte("my file contents")

	var b bytes.Buffer
	src := itermultipart.NewSource(itermultipart.PartSeq(
		itermultipart.NewPart().SetFormName("myfile").SetFileName("my-file.txt").SetContentBytes(fileContents),
		itermultipart.NewPart().SetFormName("key").SetContentString("val"),
	))
	if _, err := b.ReadFrom(src); err != nil {
		t.Fatalf("ReadFrom: unexpected error %s", err)
	}

	s := b.String()
	if len(s) == 0 {
		t.Fatal("String: unexpected empty result")
	}
	if s[0] == '\r' || s[0] == '\n' {
		t.Fatal("String: unexpected newline")
	}

	// test with standard multipart reader
	r := multipart.NewReader(&b, src.Boundary())

	part, err := r.NextPart()
	if err != nil {
		t.Fatalf("part 1: %v", err)
	}
	if g, e := part.FormName(), "myfile"; g != e {
		t.Errorf("part 1: want form name %q, got %q", e, g)
	}
	slurp, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("part 1: ReadAll: %v", err)
	}
	if e, g := string(fileContents), string(slurp); e != g {
		t.Errorf("part 1: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if err != nil {
		t.Fatalf("part 2: %v", err)
	}
	if g, e := part.FormName(), "key"; g != e {
		t.Errorf("part 2: want form name %q, got %q", e, g)
	}
	slurp, err = io.ReadAll(part)
	if err != nil {
		t.Fatalf("part 2: ReadAll: %v", err)
	}
	if e, g := "val", string(slurp); e != g {
		t.Errorf("part 2: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if part != nil || err == nil {
		t.Fatalf("expected end of parts; got %v, %v", part, err)
	}
}

func TestSourceWriteTo(t *testing.T) {
	fileContents := []byte("my file contents")

	var b bytes.Buffer
	src := itermultipart.NewSource(itermultipart.PartSeq(
		itermultipart.NewPart().SetFormName("myfile").SetFileName("my-file.txt").SetContentBytes(fileContents),
		itermultipart.NewPart().SetFormName("key").SetContentString("val"),
	))
	if _, err := src.WriteTo(&b); err != nil {
		t.Fatalf("WriteTo: unexpected error %s", err)
	}

	s := b.String()
	if len(s) == 0 {
		t.Fatal("String: unexpected empty result")
	}
	if s[0] == '\r' || s[0] == '\n' {
		t.Fatal("String: unexpected newline")
	}

	// test with standard multipart reader
	r := multipart.NewReader(&b, src.Boundary())

	part, err := r.NextPart()
	if err != nil {
		t.Fatalf("part 1: %v", err)
	}
	if g, e := part.FormName(), "myfile"; g != e {
		t.Errorf("part 1: want form name %q, got %q", e, g)
	}
	slurp, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("part 1: ReadAll: %v", err)
	}
	if e, g := string(fileContents), string(slurp); e != g {
		t.Errorf("part 1: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if err != nil {
		t.Fatalf("part 2: %v", err)
	}
	if g, e := part.FormName(), "key"; g != e {
		t.Errorf("part 2: want form name %q, got %q", e, g)
	}
	slurp, err = io.ReadAll(part)
	if err != nil {
		t.Fatalf("part 2: ReadAll: %v", err)
	}
	if e, g := "val", string(slurp); e != g {
		t.Errorf("part 2: want contents %q, got %q", e, g)
	}

	part, err = r.NextPart()
	if part != nil || err == nil {
		t.Fatalf("expected end of parts; got %v, %v", part, err)
	}
}

func TestSourceSetBoundary(t *testing.T) {
	tests := []struct {
		b  string
		ok bool
	}{
		{"abc", true},
		{"", false},
		{"ungültig", false},
		{"!", false},
		{strings.Repeat("x", 70), true},
		{strings.Repeat("x", 71), false},
		{"bad!ascii!", false},
		{"my-separator", true},
		{"with space", true},
		{"badspace ", false},
		{"(boundary)", true},
	}
	for i, tt := range tests {
		var b bytes.Buffer
		src := itermultipart.NewSource(itermultipart.PartSeq())
		err := src.SetBoundary(tt.b)
		got := err == nil
		if got != tt.ok {
			t.Errorf("%d. boundary %q = %v (%v); want %v", i, tt.b, got, err, tt.ok)
		} else if tt.ok {
			got := src.Boundary()
			if got != tt.b {
				t.Errorf("boundary = %q; want %q", got, tt.b)
			}

			ct := src.FormDataContentType()
			mt, params, err := mime.ParseMediaType(ct)
			if err != nil {
				t.Errorf("could not parse Content-Type %q: %v", ct, err)
			} else if mt != "multipart/form-data" {
				t.Errorf("unexpected media type %q; want %q", mt, "multipart/form-data")
			} else if b := params["boundary"]; b != tt.b {
				t.Errorf("unexpected boundary parameter %q; want %q", b, tt.b)
			}

			if _, err := b.ReadFrom(src); err != nil {
				t.Fatalf("ReadFrom: unexpected error %s", err)
			}
			wantSub := "\r\n--" + tt.b + "--\r\n"
			if got := b.String(); !strings.Contains(got, wantSub) {
				t.Errorf("expected %q in output. got: %q", wantSub, got)
			}
		}
	}
}

func TestSourceBoundaryGoroutines(t *testing.T) {
	// Verify there's no data race accessing any lazy boundary if it's used by
	// different goroutines.
	src := itermultipart.NewSource(itermultipart.PartSeq(
		itermultipart.NewPart().SetContentString("foo"),
	))
	done := make(chan int)
	go func() {
		new(bytes.Buffer).ReadFrom(src)
		done <- 1
	}()
	src.Boundary()
	<-done
}

func TestSortedHeader(t *testing.T) {
	header := textproto.MIMEHeader{
		"A": {"2"},
		"B": {"5", "7", "6"},
		"C": {"4"},
		"M": {"3"},
		"Z": {"1"},
	}

	src := itermultipart.NewSource(itermultipart.PartSeq(
		(&itermultipart.Part{Header: header}).SetContentString("foo"),
	))
	if err := src.SetBoundary("MIMEBOUNDARY"); err != nil {
		t.Fatalf("Error setting mime boundary: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(src); err != nil {
		t.Fatalf("ReadFrom: unexpected error %s", err)
	}

	want := "--MIMEBOUNDARY\r\nA: 2\r\nB: 5\r\nB: 7\r\nB: 6\r\nC: 4\r\nM: 3\r\nZ: 1\r\n\r\nfoo\r\n--MIMEBOUNDARY--\r\n"
	if want != buf.String() {
		t.Fatalf("\n got: %q\nwant: %q\n", buf.String(), want)
	}
}
