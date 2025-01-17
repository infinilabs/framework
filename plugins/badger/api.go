// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	log "github.com/cihub/seelog"
	"github.com/dgraph-io/badger/v4"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
	"sort"
)

const (
	SortBySize  = "size"  // Constant to specify sorting by size
	SortByCount = "count" // Constant to specify sorting by count
)

// dumpKeyStats handles the HTTP request for dumping key statistics from the Badger DB
func (m *Module) dumpKeyStats(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var stats []*KeyStats
	size := m.GetIntOrDefault(req, "size", 10)
	sortKey := m.GetParameterOrDefault(req, "sort", SortBySize)

	// Initialize totalKeyCount to count the total number of keys across all buckets
	var totalKeyCount int

	// Iterate through all the buckets in the Range
	buckets.Range(func(key, value any) bool {
		// Extract the Badger DB instance from the bucket's value
		db, ok := value.(*badger.DB)
		if !ok {
			log.Errorf("failed to get badger db for bucket %s", key)
			return true
		} else {
			if db == nil {
				log.Debugf("got empty badger db for bucket %s", key)
				return true
			}
		}

		// Get statistics for the current bucket, with key count and sorting option
		partStats, keyCount, err := getBadgerStats(db, size, sortKey)
		if err != nil {
			log.Errorf("failed to get badger stats: %v", err)
			return true
		}
		totalKeyCount += keyCount
		if len(partStats) > 0 {
			stats = append(stats, partStats...)
		}
		return true
	})

	// Sort the gathered statistics based on the specified "sort" parameter
	if sortKey == SortBySize {
		sort.Sort(BySize(stats)) // Sort by value size
	} else {
		sort.Sort(ByCount(stats)) // Sort by count
	}

	// Ensure we do not slice beyond the length of stats
	if len(stats) < size {
		size = len(stats) // If requested size is greater than the available stats, adjust size
	}

	w.Header().Set("Content-Type", "application/json")

	w.Write(util.MustToJSONBytes(util.MapStr{
		"total": totalKeyCount,       // Total number of keys across all buckets
		"top_hits":           stats[:size], // The top N key statistics based on size or count
	}))
	w.WriteHeader(200)
}


// KeyStats represents the statistics for each key, including its occurrence count and value size.
type KeyStats struct {
	Key   string // The key itself
	Count int    // The number of times the key appears in the database
	Size  int64    // The size of the key's associated value
}

func getBadgerStats(db *badger.DB, size int, sortBy string) ([]*KeyStats, int, error) {

	keyStats := make(map[string]*KeyStats)

	// Perform a read-only transaction
	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close() // Make sure the iterator is closed when done

		// Iterate through all the items in the database
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			// Count the occurrence of the current key
			if _, ok := keyStats[key]; !ok {
				keyStats[key] = &KeyStats{
					Key:   key,
					Count: 1,
					Size:  item.ValueSize(),
				}
			}else{
				keyStats[key].Count++
				keyStats[key].Size += item.ValueSize()
			}
		}
		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	// Create a slice to store the statistics of each key
	var stats = make([]*KeyStats, 0, len(keyStats))
	for _, v := range keyStats {
		stats = append(stats, v)
	}
	keyCount := len(stats)
	// Sort the keys based on their occurrence count or value size
	if sortBy == SortBySize {
		sort.Sort(BySize(stats))
	}else {
		sort.Sort(ByCount(stats))
	}
	// Ensure we do not slice beyond the length of stats
	if len(stats) < size {
		size = len(stats)
	}
	return stats[0: size], keyCount, nil
}

// ByCount implements sort.Interface to sort KeyStats by the occurrence count of each key.
type ByCount []*KeyStats

func (a ByCount) Len() int           { return len(a) }
func (a ByCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCount) Less(i, j int) bool { return a[i].Count > a[j].Count } // Sort in descending order of count

// BySize implements sort.Interface to sort KeyStats by the size of the value for each key.
type BySize []*KeyStats

func (a BySize) Len() int           { return len(a) }
func (a BySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool { return a[i].Size > a[j].Size } // Sort in descending order of value size