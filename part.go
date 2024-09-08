package itermultipart

import (
	"bufio"
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
)

var emptyParams = make(map[string]string)

const (
	contentDispositionHeader = "Content-Disposition"
	contentTypeHeader        = "Content-Type"
	formDataDisposition      = "form-data"
)

// Part represents a part of a multipart message.
type Part struct {
	Header  textproto.MIMEHeader
	Content io.Reader

	disposition       string
	dispositionParams map[string]string
}

// NewPart creates a new part.
func NewPart() *Part {
	return &Part{
		Header: make(textproto.MIMEHeader),
	}
}

// SetFormName sets the form name of the part.
func (p *Part) SetFormName(formName string) *Part {
	if p.dispositionParams == nil {
		p.dispositionParams = make(map[string]string)
	}
	p.dispositionParams["name"] = formName
	p.disposition = mime.FormatMediaType(formDataDisposition, p.dispositionParams)
	p.Header.Set(contentDispositionHeader, p.disposition)
	return p
}

// FormName returns the name parameter if p has a Content-Disposition
// of type "form-data".  Otherwise, it returns the empty string.
func (p *Part) FormName() string {
	// See https://tools.ietf.org/html/rfc2183 section 2 for EBNF
	// of Content-Disposition value format.
	p.parseContentDisposition()
	if p.disposition != formDataDisposition {
		return ""
	}
	return p.dispositionParams["name"]
}

// SetFileName sets the file name of the part.
// It also sets the "Content-Type" header to "application/octet-stream" like [multipart.Writer.CreateFormFile].
func (p *Part) SetFileName(fileName string) *Part {
	p.dispositionParams["filename"] = fileName
	p.disposition = mime.FormatMediaType(formDataDisposition, p.dispositionParams)
	p.Header.Set(contentDispositionHeader, p.disposition)
	// Go's standard multipart.Writer does this when you create a file part
	p.Header.Set(contentTypeHeader, "application/octet-stream")
	return p
}

// FileName returns the filename parameter of the [Part]'s Content-Disposition
// header. If not empty, the filename is passed through filepath.Base (which is
// platform dependent) before being returned.
func (p *Part) FileName() string {
	p.parseContentDisposition()
	filename := p.dispositionParams["filename"]
	if filename == "" {
		return ""
	}
	// RFC 7578, Section 4.2 requires that if a filename is provided, the
	// directory path information must not be used.
	return filepath.Base(filename)
}

// SetContent sets the content of the part.
func (p *Part) SetContent(content io.Reader) *Part {
	p.Content = content
	return p
}

// SetContentString sets the content of the part to the given string.
func (p *Part) SetContentString(content string) *Part {
	if sr, ok := p.Content.(*strings.Reader); ok {
		sr.Reset(content)
		return p
	}

	return p.SetContent(strings.NewReader(content))
}

// SetContentBytes sets the content of the part to the given bytes.
func (p *Part) SetContentBytes(content []byte) *Part {
	if br, ok := p.Content.(*bytes.Reader); ok {
		br.Reset(content)
		return p
	}
	return p.SetContent(bytes.NewReader(content))
}

// SetContentType sets the content type of the part.
func (p *Part) SetContentType(contentType string) *Part {
	if p.Header == nil {
		p.Header = make(textproto.MIMEHeader)
	}
	p.Header.Set(contentTypeHeader, contentType)
	return p
}

// ContentType returns the content type of the part.
func (p *Part) ContentType() string {
	return p.Header.Get(contentTypeHeader)
}

// DetectContentType detects the content type of the part using [net/http.DetectContentType].
// It peeks the first 512 bytes of the content to determine the content type.
// Content must be already set before calling this method.
// If content-type cannot be detected, it sets the content type to "application/octet-stream".
// Note that this method modifies Content field of the part.
func (p *Part) DetectContentType() *Part {
	const sniffLen = 512
	br := bufio.NewReaderSize(p.Content, sniffLen)
	// it's safe to ignore error here because error sticks internally to reader and returns on the next read
	signature, _ := br.Peek(sniffLen)

	return p.SetContent(br).SetContentType(http.DetectContentType(signature))
}

// SetContentTypeByExtension sets the content type of the part based on the file extension.
// If the file name was not set, it does nothing.
// The content type is set using [mime.TypeByExtension] so you can register custom types using [mime.AddExtensionType].
func (p *Part) SetContentTypeByExtension() *Part {
	if p.FileName() == "" {
		return p
	}

	typ := mime.TypeByExtension(filepath.Ext(p.FileName()))
	if typ != "" {
		return p.SetContentType(typ)
	}

	return p
}

// SetHeaderValue sets the value of the given header key.
func (p *Part) SetHeaderValue(key, value string) *Part {
	if p.Header == nil {
		p.Header = make(textproto.MIMEHeader)
	}
	p.Header.Set(key, value)
	return p
}

// AddHeaderValue adds the value to the given header key.
func (p *Part) AddHeaderValue(key, value string) *Part {
	if p.Header == nil {
		p.Header = make(textproto.MIMEHeader)
	}
	p.Header.Add(key, value)
	return p
}

// MergeHeaders merges the given headers into the part's headers.
func (p *Part) MergeHeaders(h textproto.MIMEHeader) *Part {
	if p.Header == nil {
		p.Header = make(textproto.MIMEHeader)
	}
	for k, v := range h {
		p.Header[k] = v
	}
	return p
}

// AddToWriter adds the part to the standard [mime/multipart.Writer].
func (p *Part) AddToWriter(mw *multipart.Writer) error {
	pw, err := mw.CreatePart(p.Header)
	if err != nil {
		return err
	}
	_, err = io.Copy(pw, p.Content)
	return err
}

// Reset resets the part to its initial state.
func (p *Part) Reset() {
	clear(p.Header)
	p.Content = nil
	p.disposition = ""
	p.dispositionParams = nil // to be able to parse again
}

func (p *Part) parseContentDisposition() {
	v := p.Header[contentDispositionHeader]
	if len(v) == 0 {
		p.disposition = ""
		p.dispositionParams = emptyParams
		return
	}

	if p.dispositionParams != nil && p.disposition == v[0] {
		// if header is already parsed, verify that it's the same
		return
	}

	var err error
	p.disposition, p.dispositionParams, err = mime.ParseMediaType(v[0])
	if err != nil {
		p.dispositionParams = emptyParams
	}
}
