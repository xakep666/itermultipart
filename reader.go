package itermultipart

import (
	"errors"
	"io"
	"iter"
	"mime/multipart"
	"net/http"
)

// PartsFromReader reads each part from the provided [multipart.Reader] and yields it to the caller.
// If raw is true, it reads the raw part using [multipart.Reader.NextRawPart].
// Note that [Part] becomes invalid on the next iteration so reference to it must not be held.
func PartsFromReader(r *multipart.Reader, raw bool) iter.Seq2[*Part, error] {
	return func(yield func(*Part, error) bool) {
		p := new(Part)
		for {
			var part *multipart.Part
			var err error

			if raw {
				part, err = r.NextRawPart()
			} else {
				part, err = r.NextPart()
			}
			switch {
			case errors.Is(err, io.EOF):
				return
			case errors.Is(err, nil):
				// pass
			default:
				yield(nil, err)
				return
			}

			p.Reset()
			p.Header = part.Header
			p.Content = part
			next := yield(p, nil)
			part.Close()
			if !next {
				return
			}
		}
	}
}

// PartsFromRequest reads each part from the http request and yields it to the caller.
// If raw is true, it reads the raw part using [multipart.Part.NextRawPart].
// Note that [Part] becomes invalid on the next iteration so reference to it must not be held.
func PartsFromRequest(r *http.Request, raw bool) iter.Seq2[*Part, error] {
	reader, err := r.MultipartReader()
	if err != nil {
		return func(yield func(*Part, error) bool) {
			yield(nil, err)
		}
	}
	return PartsFromReader(reader, raw)
}
