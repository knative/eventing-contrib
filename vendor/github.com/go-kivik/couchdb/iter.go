package couchdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/go-kivik/kivik"
)

type parser interface {
	decodeItem(interface{}, *json.Decoder) error
}

type metaParser interface {
	parseMeta(interface{}, *json.Decoder, string) error
}

type cancelableReadCloser struct {
	ctx    context.Context
	err    error
	rc     io.ReadCloser
	closed bool
	mu     sync.RWMutex
}

var _ io.ReadCloser = &cancelableReadCloser{}

func newCancelableReadCloser(ctx context.Context, rc io.ReadCloser) io.ReadCloser {
	return &cancelableReadCloser{ctx: ctx, rc: rc}
}

func (r *cancelableReadCloser) Read(p []byte) (int, error) {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return 0, r.close(nil)
	}
	r.mu.RUnlock()
	var c int
	var err error
	done := make(chan struct{})
	go func() {
		c, err = r.rc.Read(p)
		close(done)
	}()
	select {
	case <-r.ctx.Done():
		return 0, r.close(r.ctx.Err())
	case <-done:
		if err != nil {
			e := r.close(err)
			return c, e
		}
		return c, nil
	}
}

func (r *cancelableReadCloser) close(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		e := r.rc.Close()
		if err == nil {
			err = e
		}
		r.err = err
	}
	return r.err
}

func (r *cancelableReadCloser) Close() error {
	err := r.close(nil)
	if err == io.EOF {
		return nil
	}
	return err
}

type iter struct {
	meta        interface{}
	expectedKey string
	body        io.ReadCloser
	parser      parser

	// objMode enables reading one object at a time, with the ID treated as the
	// docid. This was added for the _revs_diff endpoint.
	objMode bool

	dec    *json.Decoder
	mu     sync.RWMutex
	closed bool
}

func newIter(ctx context.Context, meta interface{}, expectedKey string, body io.ReadCloser, parser parser) *iter {
	return &iter{
		meta:        meta,
		expectedKey: expectedKey,
		body:        newCancelableReadCloser(ctx, body),
		parser:      parser,
	}
}

func (i *iter) next(row interface{}) error {
	i.mu.RLock()
	if i.closed {
		i.mu.RUnlock()
		return io.EOF
	}
	i.mu.RUnlock()
	if i.dec == nil {
		// We haven't begun yet
		i.dec = json.NewDecoder(i.body)
		if err := i.begin(); err != nil {
			return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: err}
		}
	}

	err := i.nextRow(row)
	if err != nil {
		if err == io.EOF {
			if e := i.finish(); e != nil {
				err = e
			}
			return err
		}
	}
	return err
}

// begin parses the top-level of the result object; until rows
func (i *iter) begin() error {
	if i.expectedKey == "" && !i.objMode {
		return nil
	}
	// consume the first '{'
	if err := consumeDelim(i.dec, json.Delim('{')); err != nil {
		return err
	}
	if i.objMode {
		return nil
	}
	for {
		t, err := i.dec.Token()
		if err != nil {
			// I can't find a test case to trigger this, so it remains uncovered.
			return err
		}
		key, ok := t.(string)
		if !ok {
			// The JSON parser should never permit this
			return fmt.Errorf("Unexpected token: (%T) %v", t, t)
		}
		if key == i.expectedKey {
			// Consume the first '['
			return consumeDelim(i.dec, json.Delim('['))
		}
		if err := i.parseMeta(key); err != nil {
			return err
		}
	}
}

func (i *iter) parseMeta(key string) error {
	if i.meta == nil {
		return nil
	}
	if mp, ok := i.parser.(metaParser); ok {
		return mp.parseMeta(i.meta, i.dec, key)
	}
	return nil
}

func (i *iter) finish() (err error) {
	defer func() {
		e2 := i.Close()
		if err == nil {
			err = e2
		}
	}()
	if i.expectedKey == "" && !i.objMode {
		_, err := i.dec.Token()
		if err != nil && err != io.EOF {
			return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: err}
		}
		return nil
	}
	if i.objMode {
		err := consumeDelim(i.dec, json.Delim('}'))
		if err != nil && err != io.EOF {
			return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: err}
		}
		return nil
	}
	if err := consumeDelim(i.dec, json.Delim(']')); err != nil {
		return err
	}
	for {
		t, err := i.dec.Token()
		if err != nil {
			return err
		}
		switch v := t.(type) {
		case json.Delim:
			if v != json.Delim('}') {
				// This should never happen, as the JSON parser should prevent it.
				return fmt.Errorf("Unexpected JSON delimiter: %c", v)
			}
		case string:
			if err := i.parseMeta(v); err != nil {
				return err
			}
		default:
			// This should never happen, as the JSON parser would never get
			// this far.
			return fmt.Errorf("Unexpected JSON token: (%T) '%s'", t, t)
		}
	}
}

func (i *iter) nextRow(row interface{}) error {
	if !i.dec.More() {
		return io.EOF
	}
	return i.parser.decodeItem(row, i.dec)
}

func (i *iter) Close() error {
	i.mu.Lock()
	i.closed = true
	i.mu.Unlock()
	return i.body.Close()
}

// consumeDelim consumes the expected delimiter from the stream, or returns an
// error if an unexpected token was found.
func consumeDelim(dec *json.Decoder, expectedDelim json.Delim) error {
	t, err := dec.Token()
	if err != nil {
		return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: err}
	}
	d, ok := t.(json.Delim)
	if !ok {
		return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: fmt.Errorf("Unexpected token %T: %v", t, t)}
	}
	if d != expectedDelim {
		return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: fmt.Errorf("Unexpected JSON delimiter: %c", d)}
	}
	return nil
}
