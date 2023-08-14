package main

// Copyright (c) 2016 Mattias Wadman

// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
// of the Software, and to permit persons to whom the Software is furnished to do
// so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import (
	"bytes"
	"io"
)

type restartBuffer struct {
	Buffer    bytes.Buffer
	Restarted bool
}

func (rb *restartBuffer) Read(r io.Reader, p []byte) (n int, err error) {
	if rb.Restarted {
		if rb.Buffer.Len() > 0 {
			return rb.Buffer.Read(p)
		}
		n, err = r.Read(p)
		return n, err
	}

	n, err = r.Read(p)
	rb.Buffer.Write(p[:n])

	return n, err
}

// ReReader transparently buffers all reads from a reader until Restarted
// is set to true. When restarted buffered data will be replayed on read and
// after that normal reading from the reader continues.
type ReReader struct {
	io.Reader
	restartBuffer
}

// NewReReader return a initialized ReReader
func NewReReader(r io.Reader) *ReReader {
	return &ReReader{Reader: r}
}

func (rr *ReReader) Read(p []byte) (n int, err error) {
	return rr.restartBuffer.Read(rr.Reader, p)
}

// ReReadCloser is same as ReReader but also forwards Close calls
type ReReadCloser struct {
	io.ReadCloser
	restartBuffer
}

// NewReReadCloser return a initialized ReReadCloser
func NewReReadCloser(rc io.ReadCloser) *ReReadCloser {
	return &ReReadCloser{ReadCloser: rc}
}

func (rc *ReReadCloser) Read(p []byte) (n int, err error) {
	return rc.restartBuffer.Read(rc.ReadCloser, p)
}
