package itermultipart

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"mime"
	"slices"
)

// Source is a generator of multipart message as you read from it.
type Source struct {
	randBoundary [30]byte                // used only on bootstraps
	boundary     string                  // used in the message
	parts        iter.Seq2[*Part, error] // for WriteTo

	pull                func() (*Part, error, bool)
	stop                func()
	buffered            *bytes.Buffer // accumulates boundary+headers
	firstHeadingWritten bool
	lastPart            *Part
	finalizing          bool
	closed              bool
}

// NewSource returns a new [Source] that generates a multipart message from provided part sequence.
// Part sequence must be finite.
// [Source] holds reference for [Part] only until it's fully read.
func NewSource(parts iter.Seq2[*Part, error]) *Source {
	src := &Source{
		parts:    parts,
		buffered: new(bytes.Buffer),
	}
	src.populateRandomBoundary()
	return src
}

func (s *Source) populateRandomBoundary() {
	_, err := io.ReadFull(rand.Reader, s.randBoundary[:])
	if err != nil {
		panic(err)
	}
	s.boundary = fmt.Sprintf("%x", s.randBoundary)
}

// PartSeq returns a sequence of parts from the provided list.
func PartSeq(parts ...*Part) iter.Seq2[*Part, error] {
	return func(yield func(*Part, error) bool) {
		for _, part := range parts {
			if !yield(part, nil) {
				return
			}
		}
	}
}

// Read implements [io.Reader].
func (s *Source) Read(p []byte) (n int, err error) {
	if s.closed {
		return 0, fmt.Errorf("source is closed")
	}

	if s.pull == nil {
		s.pull, s.stop = iter.Pull2(s.parts)
	}

	// pull the next part if necessary
	if s.lastPart == nil && !s.finalizing {
		part, err, ok := s.pull()
		if !ok {
			// finalize
			s.finalizing = true
			return s.populateEnding().Read(p)
		}
		if err != nil {
			return 0, err
		}
		s.lastPart = part
		s.populatePartHeading(part)
	}

	if s.buffered.Len() > 0 {
		// we have some buffered data, read it first
		bufRead, bufReadErr := s.buffered.Read(p)
		switch {
		case errors.Is(bufReadErr, nil):
			n += bufRead
			p = p[bufRead:]
		case errors.Is(bufReadErr, io.EOF):
			// continue reading parts
		default:
			return bufRead, bufReadErr
		}
	}

	if s.finalizing {
		if n > 0 {
			// do not return EOF if we read some data
			return n, nil
		}
		return 0, io.EOF
	}

	// read the content of the last part
	readSize, readErr := s.lastPart.Content.Read(p)
	n += readSize
	if errors.Is(readErr, io.EOF) {
		s.lastPart = nil // prepare for the next part
		return n, nil
	}

	return n, readErr
}

// WriteTo implements the [io.WriterTo] interface allowing some source-target optimizations to be used.
func (s *Source) WriteTo(target io.Writer) (int64, error) {
	if s.closed {
		return 0, fmt.Errorf("source is closed")
	}

	var n int64
	for part, err := range s.parts {
		if err != nil {
			return n, err
		}

		// write part heading
		partHeadingSize, err := s.populatePartHeading(part).WriteTo(target)
		n += partHeadingSize
		if err != nil {
			return n, err
		}

		contentSize, err := s.writePartContent(part, target)
		n += contentSize
		if err != nil {
			return n, err
		}
	}

	// it's last part, so we must finalize
	endSize, err := s.populateEnding().WriteTo(target)
	n += endSize
	return n, err
}

func (s *Source) writePartContent(part *Part, target io.Writer) (int64, error) {
	// if ReaderFrom or WriterTo is implemented, use it. Checking order matches io.Copy.
	if wt, ok := part.Content.(io.WriterTo); ok {
		return wt.WriteTo(target)
	}
	if rf, ok := target.(io.ReaderFrom); ok {
		return rf.ReadFrom(part.Content)
	}

	// allocate or reuse buffer for copying
	bufferSize := 32 * 1024 // default value from io.CopyBuffer
	if l, ok := part.Content.(*io.LimitedReader); ok && int64(bufferSize) > l.N {
		if l.N < 1 {
			bufferSize = 1
		} else {
			bufferSize = int(l.N)
		}
	}
	s.buffered.Reset()
	s.buffered.Grow(bufferSize)

	// copy content
	return io.CopyBuffer(target, part.Content, s.buffered.Bytes())
}

func (s *Source) populatePartHeading(part *Part) *bytes.Buffer {
	s.buffered.Reset()
	if !s.firstHeadingWritten {
		s.firstHeadingWritten = true
		s.buffered.WriteString("--")
	} else {
		s.buffered.WriteString("\r\n--")
	}
	s.buffered.WriteString(s.boundary)
	for _, k := range slices.Sorted(maps.Keys(part.Header)) {
		for _, v := range part.Header[k] {
			s.buffered.WriteString("\r\n")
			s.buffered.WriteString(k)
			s.buffered.WriteString(": ")
			s.buffered.WriteString(v)
		}
	}
	s.buffered.WriteString("\r\n\r\n")
	return s.buffered
}

func (s *Source) populatePartEnding() *bytes.Buffer {
	s.buffered.Reset()
	s.buffered.WriteString("\r\n")
	return s.buffered
}

func (s *Source) populateEnding() *bytes.Buffer {
	s.buffered.Reset()
	s.buffered.WriteString("\r\n--")
	s.buffered.WriteString(s.boundary)
	s.buffered.WriteString("--\r\n")
	return s.buffered
}

// SetBoundary overrides the [Source]'s default randomly-generated
// boundary separator with an explicit value.
//
// SetBoundary must be called before any parts are created, may only
// contain certain ASCII characters, and must be non-empty and
// at most 70 bytes long.
func (s *Source) SetBoundary(boundary string) error {
	if s.lastPart != nil {
		return errors.New("SetBoundary called after read")
	}
	// rfc2046#section-5.1.1
	if len(boundary) < 1 || len(boundary) > 70 {
		return errors.New("invalid boundary length")
	}
	end := len(boundary) - 1
	for i, b := range boundary {
		if 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z' || '0' <= b && b <= '9' {
			continue
		}
		switch b {
		case '\'', '(', ')', '+', '_', ',', '-', '.', '/', ':', '=', '?':
			continue
		case ' ':
			if i != end {
				continue
			}
		}
		return errors.New("invalid boundary character")
	}
	s.boundary = boundary
	return nil
}

// FormDataContentType returns the Content-Type for an HTTP
// multipart/form-data with this [Source]'s Boundary.
func (s *Source) FormDataContentType() string {
	return mime.FormatMediaType("multipart/form-data", map[string]string{"boundary": s.boundary})
}

// Boundary returns the [Source]'s boundary.
func (s *Source) Boundary() string {
	return s.boundary
}

// Close closes the [Source], preventing further reads.
func (s *Source) Close() error {
	if s.stop != nil {
		s.stop()
	}
	s.boundary = ""
	s.buffered.Reset()
	s.firstHeadingWritten = false
	s.finalizing = false
	s.lastPart = nil
	s.closed = true
	return nil
}

// Reset resets the [Source] to use the provided part sequence.
func (s *Source) Reset(parts iter.Seq2[*Part, error]) {
	if s.stop != nil {
		s.stop()
	}
	s.populateRandomBoundary()
	s.parts = parts
	s.buffered.Reset()
	s.firstHeadingWritten = false
	s.finalizing = false
	s.lastPart = nil
	s.closed = false
}

type errorReader struct {
	err error
}

func (r *errorReader) Read([]byte) (n int, err error) {
	return 0, r.err
}
