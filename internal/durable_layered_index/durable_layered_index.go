package durableindex

import (
	"context"
)

type DurableLayeredIndex struct {
	dbCollection     any
	vector_bin_index int
	vectorBinMap     map[int]*Bin
}

func New(dbCollection any) *DurableLayeredIndex {
	d := &DurableLayeredIndex{
		vectorBinMap: make(map[int]*Bin),
	}

	// Create 1 random bin
	d.vectorBinMap[0] = NewBin(dbCollection)

	return d
}

func (d *DurableLayeredIndex) Query(ctx context.Context, text string) (string, error) {
	resultChan := make(chan string, 1)
	query := NewQuery(ctx, text)

	// For now: always send to bin 0
	selectedBin := d.vectorBinMap[0]

	// async
	go selectedBin.AddQuery(query)

	// wait for result
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultChan:
		return result, nil
	}
}
