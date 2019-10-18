package kivikmock

import (
	"context"

	"github.com/go-kivik/kivik/driver"
)

type driverDB struct {
	*DB
}

var _ driver.DB = &driverDB{}
var _ driver.BulkGetter = &driverDB{}
var _ driver.Finder = &driverDB{}

func (db *driverDB) Close(ctx context.Context) error {
	expected := &ExpectedDBClose{
		commonExpectation: commonExpectation{db: db.DB},
	}
	if err := db.client.nextExpectation(expected); err != nil {
		return err
	}
	if expected.callback != nil {
		return expected.callback(ctx)
	}
	return expected.wait(ctx)
}
