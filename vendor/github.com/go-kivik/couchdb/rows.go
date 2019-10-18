package couchdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-kivik/kivik"
	"github.com/go-kivik/kivik/driver"
)

type rowsMeta struct {
	offset    int64
	totalRows int64
	updateSeq sequenceID
	warning   string
	bookmark  string
}

type rows struct {
	*iter
	*rowsMeta
}

var _ driver.Rows = &rows{}

type rowsMetaParser struct{}

func (p *rowsMetaParser) parseMeta(i interface{}, dec *json.Decoder, key string) error {
	meta := i.(*rowsMeta)
	return meta.parseMeta(key, dec)
}

type rowParser struct {
	rowsMetaParser
}

var _ parser = &rowParser{}

func (p *rowParser) decodeItem(i interface{}, dec *json.Decoder) error {
	return dec.Decode(i)
}

func newRows(ctx context.Context, in io.ReadCloser) driver.Rows {
	meta := &rowsMeta{}
	return &rows{
		iter:     newIter(ctx, meta, "rows", in, &rowParser{}),
		rowsMeta: meta,
	}
}

type findParser struct {
	rowsMetaParser
}

var _ parser = &findParser{}

func (p *findParser) decodeItem(i interface{}, dec *json.Decoder) error {
	row := i.(*driver.Row)
	return dec.Decode(&row.Doc)
}

func newFindRows(ctx context.Context, in io.ReadCloser) driver.Rows {
	meta := &rowsMeta{}
	return &rows{
		iter:     newIter(ctx, meta, "docs", in, &findParser{}),
		rowsMeta: meta,
	}
}

type bulkParser struct {
	rowsMetaParser
}

var _ parser = &bulkParser{}

func (p *bulkParser) decodeItem(i interface{}, dec *json.Decoder) error {
	row := i.(*driver.Row)
	var result bulkResult
	if err := dec.Decode(&result); err != nil {
		return err
	}
	row.ID = result.ID
	row.Doc = result.Docs[0].Doc
	row.Error = nil
	if err := result.Docs[0].Error; err != nil {
		row.Error = err
	}
	return nil
}

func newBulkGetRows(ctx context.Context, in io.ReadCloser) driver.Rows {
	meta := &rowsMeta{}
	return &rows{
		iter:     newIter(ctx, meta, "results", in, &bulkParser{}),
		rowsMeta: meta,
	}
}

func (r *rows) Offset() int64 {
	return r.offset
}

func (r *rows) TotalRows() int64 {
	return r.totalRows
}

func (r *rows) Warning() string {
	return r.warning
}

func (r *rows) Bookmark() string {
	return r.bookmark
}

func (r *rows) UpdateSeq() string {
	return string(r.updateSeq)
}

func (r *rows) Next(row *driver.Row) error {
	return r.iter.next(row)
}

// parseMeta parses result metadata
func (r *rowsMeta) parseMeta(key string, dec *json.Decoder) error {
	switch key {
	case "update_seq":
		return dec.Decode(&r.updateSeq)
	case "offset":
		return dec.Decode(&r.offset)
	case "total_rows":
		return dec.Decode(&r.totalRows)
	case "warning":
		return dec.Decode(&r.warning)
	case "bookmark":
		return dec.Decode(&r.bookmark)
	}
	return &kivik.Error{HTTPStatus: http.StatusBadGateway, Err: fmt.Errorf("Unexpected key: %s", key)}
}
