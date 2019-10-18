package kivikmock

import (
	"context"
	"time"

	"github.com/go-kivik/kivik/driver"
)

// BulkResults is a mocked collection of BulkDoc results.
type BulkResults struct {
	iter
}

type driverBulkResults struct {
	context.Context
	*BulkResults
}

var _ driver.BulkResults = &driverBulkResults{}

func (r *driverBulkResults) Next(res *driver.BulkResult) error {
	result, err := r.unshift(r.Context)
	if err != nil {
		return err
	}
	*res = *result.(*driver.BulkResult)
	return nil
}

// CloseError sets an error to be returned when the iterator is closed.
func (r *BulkResults) CloseError(err error) *BulkResults {
	r.closeErr = err
	return r
}

// AddResult adds a bulk result to be returned by the iterator. If
// AddResultError has been set, this method will panic.
func (r *BulkResults) AddResult(result *driver.BulkResult) *BulkResults {
	if r.resultErr != nil {
		panic("It is invalid to set more results after AddResultError is defined.")
	}
	r.push(&item{item: result})
	return r
}

// AddResultError adds an error to be returned during iteration.
func (r *BulkResults) AddResultError(err error) *BulkResults {
	r.resultErr = err
	return r
}

// AddDelay adds a delay before the next iteration will complete.
func (r *BulkResults) AddDelay(delay time.Duration) *BulkResults {
	r.push(&item{delay: delay})
	return r
}

// Final converts the BulkResults object to a driver.BulkResults. This method
// is intended for use within WillExecute() to return results.
func (r *BulkResults) Final() driver.BulkResults {
	return &driverBulkResults{BulkResults: r}
}
