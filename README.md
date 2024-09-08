itermultipart - A convenient way to work with `multipart/form-data` messages using iterators
=======

[![Build Status](https://github.com/xakep666/itermultipart/actions/workflows/testing.yml/badge.svg)](https://github.com/xakep666/itermultipart/actions/workflows/testing.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xakep666/itermultipart?service=github)](https://goreportcard.com/report/github.com/xakep666/itermultipart)
[![Go Reference](https://pkg.go.dev/badge/github.com/xakep666/itermultipart.svg)](https://pkg.go.dev/github.com/xakep666/itermultipart)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Itermultipart simplifies reading of multipart messages by providing an iterator interface to [multipart.Reader](https://golang.org/pkg/mime/multipart/#Reader)
and creating multipart messages from [iterators](https://pkg.go.dev/iter#hdr-Iterators).

## Features
* [Functions](#reading-parts) to convert `multipart.Reader` or `http.Request` to an iterator suitable for `for range` loop
* [Generate](#creating-http-request) multipart messages from iterators. Message generator implements `io.Reader` interface, so it's suitable for http request body directly.
* Convenient parts [constructor](#creating-parts) with fluent interface
* Zero-dependency

## Reading parts

Reading parts via wrapped `multipart.Reader` is an efficient way to work with multipart messages.
This also allows to be more flexible with limitation of message/part size, parts count and memory usage.

```go
func SimpleFileSaveHandler(w http.ResponseWriter, *r http.Request) {
	for part, err := range itermultipart.PartsFromRequest(r) {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Save part to file
		if part.FileName() == "" {
			http.Error(w, "File name is required", http.StatusBadRequest)
			return
		}

		file, err := os.Create(part.FileName())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		_, err = io.Copy(file, part.Content)
		file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
```

Also, you can feed standard `multipart.Reader` to [itermultipart.PartsFromReader](https://pkg.go.dev/github.com/xakep666/itermultipart#PartsFromReader) function.

## Creating HTTP request

Traditional way with `multipart.Writer`:
```go
func CreateMultipartRequest() (*http.Request, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	
	go func() {
		// Write parts
		part, err := mw.CreateFormFile("file", "file.txt")
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		
		_, err = part.Write([]byte("Hello, world!"))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		
		pw.CloseWithError(mw.Close())
	}()
	
	req, err := http.NewRequest("POST", "http://example.com/upload", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	
	return req, nil
}
```

As you may notice it requires extra goroutine and `io.Pipe` to work with `multipart.Writer`.
This makes impossible to use some optimizations provided by [io.ReaderFrom](https://pkg.go.dev/io#ReaderFrom) or [io.WriterTo](https://pkg.go.dev/io#WriterTo) interfaces i.e. direct file-socket or socket-socket transfer.

However, with you can use `itermultipart.Source` to create multipart message from iterator:
```go
func CreateMultipartRequest() (*http.Request, error) {
	src := itermultipart.NewSource(itermultipart.PartSeq(
		itermultipart.NewPart().
			SetFormName("file").
			SetFileName("file.txt").
			SetContentString("Hello, world!"),
	))
	req, err := http.NewRequest("POST", "http://example.com/upload", src)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", src.FormDataContentType())
	return req, nil
}
```

As you can see its much simpler and doesn't require extra goroutine and `io.Pipe`.
`PartSeq` here is a simple helper that transforms list of parts to iterator.

## Creating parts

`itermultipart.NewPart` provides a fluent interface to create parts:
```go
part := itermultipart.NewPart().
	SetFormName("file").
	SetFileName("file.txt").
	SetContentString("Hello, world!")
```

Content may be set via methods:
* `SetContent` - set content directly from `io.Reader`
* `SetContentString` - use provided string as content
* `SetContentBytes` - use provided byte slice as content

To define a content-type you can use:
* `SetContentType` - set content type directly
* `SetContentTypeByExtension` - set content type by file extension if `SetFileName` called before
* `DetectContentType` - peeks first 512 bytes from content and tries to recognize content type

Even if you don't want to use `itermultipart.Source` to create a multipart message, `itermultipart.NewPart` still may be useful.
It's method `AddToWriter` allows to add part to standard `multipart.Writer`:
```go
func WritePartToMultipartWriter(w *multipart.Writer) error {
	return itermultipart.NewPart().
		SetFormName("file").
		SetFileName("file.txt").
		SetContentString("Hello, world!").
		AddToWriter(w)
}
```
