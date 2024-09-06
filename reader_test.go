package itermultipart_test

import (
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"slices"
	"strings"

	"github.com/xakep666/itermultipart"
)

func ExampleParts() {
	message := `--boundary
Content-Disposition: form-data; name="myfile"; filename="example.txt"

contents of myfile
--boundary
Content-Disposition: form-data; name="key"

value for key
--boundary--`
	message = strings.ReplaceAll(message, "\n", "\r\n")
	reader := multipart.NewReader(strings.NewReader(message), "boundary")

	for part, err := range itermultipart.Parts(reader, false) {
		if err != nil {
			panic(err)
		}
		if part == nil {
			continue
		}

		fmt.Println("---headers---")
		for _, k := range slices.Sorted(maps.Keys(part.Header)) {
			fmt.Printf("%s: %s\n", k, part.Header[k])
		}
		fmt.Println("---identifiers---")
		if part.FormName() != "" {
			fmt.Println("name:", part.FormName())
		}
		if part.FileName() != "" {
			fmt.Println("filename:", part.FileName())
		}
		fmt.Println("---content---")
		io.Copy(os.Stdout, part.Content)
		fmt.Println()
	}
	// Output:
	// ---headers---
	// Content-Disposition: [form-data; name="myfile"; filename="example.txt"]
	// ---identifiers---
	// name: myfile
	// filename: example.txt
	// ---content---
	// contents of myfile
	// ---headers---
	// Content-Disposition: [form-data; name="key"]
	// ---identifiers---
	// name: key
	// ---content---
	// value for key
}

func ExamplePartsFromRequest() {
	message := `--boundary
Content-Disposition: form-data; name="myfile"; filename="example.txt"

contents of myfile
--boundary
Content-Disposition: form-data; name="key"

value for key
--boundary--`
	message = strings.ReplaceAll(message, "\n", "\r\n")
	r := httptest.NewRequest("POST", "/", strings.NewReader(message))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	for part, err := range itermultipart.PartsFromRequest(r, false) {
		if err != nil {
			panic(err)
		}
		if part == nil {
			continue
		}

		fmt.Println("---headers---")
		for _, k := range slices.Sorted(maps.Keys(part.Header)) {
			fmt.Printf("%s: %s\n", k, part.Header[k])
		}
		fmt.Println("---identifiers---")
		if part.FormName() != "" {
			fmt.Println("name:", part.FormName())
		}
		if part.FileName() != "" {
			fmt.Println("filename:", part.FileName())
		}
		fmt.Println("---content---")
		io.Copy(os.Stdout, part.Content)
		fmt.Println()
	}
	// Output:
	// ---headers---
	// Content-Disposition: [form-data; name="myfile"; filename="example.txt"]
	// ---identifiers---
	// name: myfile
	// filename: example.txt
	// ---content---
	// contents of myfile
	// ---headers---
	// Content-Disposition: [form-data; name="key"]
	// ---identifiers---
	// name: key
	// ---content---
	// value for key
}
