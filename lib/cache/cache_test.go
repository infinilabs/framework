package ccache

import (
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Cache_DeletesAValue(t *testing.T) {
	cache := New(Configure())
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("spice", "flow", time.Minute)
	cache.Set("worm", "sand", time.Minute)
	assert.Equal(t, cache.ItemCount(), 2)

	cache.Delete("spice")
	assert.Nil(t, cache.Get("spice"))
	assert.Equal(t, cache.Get("worm").Value(), "sand")
	assert.Equal(t, cache.ItemCount(), 1)
}

func Test_Cache_DeletesAPrefix(t *testing.T) {
	cache := New(Configure())
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("aaa", "1", time.Minute)
	cache.Set("aab", "2", time.Minute)
	cache.Set("aac", "3", time.Minute)
	cache.Set("ac", "4", time.Minute)
	cache.Set("z5", "7", time.Minute)
	assert.Equal(t, cache.ItemCount(), 5)

	assert.Equal(t, cache.DeletePrefix("9a"), 0)
	assert.Equal(t, cache.ItemCount(), 5)

	assert.Equal(t, cache.DeletePrefix("aa"), 3)
	assert.Nil(t, cache.Get("aaa"))
	assert.Nil(t, cache.Get("aab"))
	assert.Nil(t, cache.Get("aac"))
	assert.Equal(t, cache.Get("ac").Value(), "4")
	assert.Equal(t, cache.Get("z5").Value(), "7")
	assert.Equal(t, cache.ItemCount(), 2)
}

func Test_Cache_DeletesAFunc(t *testing.T) {
	cache := New(Configure())
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("a", 1, time.Minute)
	cache.Set("b", 2, time.Minute)
	cache.Set("c", 3, time.Minute)
	cache.Set("d", 4, time.Minute)
	cache.Set("e", 5, time.Minute)
	cache.Set("f", 6, time.Minute)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item) bool {
		return false
	}), 0)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item) bool {
		return item.Value().(int) < 4
	}), 3)
	assert.Equal(t, cache.ItemCount(), 3)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item) bool {
		return key == "d"
	}), 1)
	assert.Equal(t, cache.ItemCount(), 2)

}

func Test_Cache_OnDeleteCallbackCalled(t *testing.T) {
	onDeleteFnCalled := int32(0)
	onDeleteFn := func(item *Item) {
		if item.key == "spice" {
			atomic.AddInt32(&onDeleteFnCalled, 1)
		}
	}

	cache := New(Configure().OnDelete(onDeleteFn))
	cache.Set("spice", "flow", time.Minute)
	cache.Set("worm", "sand", time.Minute)

	time.Sleep(time.Millisecond * 10) // Run once to init
	cache.Delete("spice")
	time.Sleep(time.Millisecond * 10) // Wait for worker to pick up deleted items

	assert.Nil(t, cache.Get("spice"))
	assert.Equal(t, cache.Get("worm").Value(), "sand")
	assert.Equal(t, atomic.LoadInt32(&onDeleteFnCalled), int32(1))
}

func Test_Cache_FetchesExpiredItems(t *testing.T) {
	cache := New(Configure())
	fn := func() (interface{}, error) { return "moo-moo", nil }

	cache.Set("beef", "moo", time.Second*-1)
	assert.Equal(t, cache.Get("beef").Value(), "moo")

	out, _ := cache.Fetch("beef", time.Second, fn)
	assert.Equal(t, out.Value(), "moo-moo")
}

func Test_Cache_GCsTheOldestItems(t *testing.T) {
	cache := New(Configure().ItemsToPrune(10))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	//let the items get promoted (and added to our list)
	time.Sleep(time.Millisecond * 10)
	gcCache(cache)
	assert.Nil(t, cache.Get("9"))
	assert.Equal(t, cache.Get("10").Value(), 10)
	assert.Equal(t, cache.ItemCount(), 490)
}

func Test_Cache_PromotedItemsDontGetPruned(t *testing.T) {
	cache := New(Configure().ItemsToPrune(10).GetsPerPromote(1))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	time.Sleep(time.Millisecond * 10) //run the worker once to init the list
	cache.Get("9")
	time.Sleep(time.Millisecond * 10)
	gcCache(cache)
	assert.Equal(t, cache.Get("9").Value(), 9)
	assert.Nil(t, cache.Get("10"))
	assert.Equal(t, cache.Get("11").Value(), 11)
}

func Test_Cache_TrackerDoesNotCleanupHeldInstance(t *testing.T) {
	cache := New(Configure().ItemsToPrune(11).Track())
	item0 := cache.TrackingSet("0", 0, time.Minute)
	for i := 1; i < 11; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	item1 := cache.TrackingGet("1")
	time.Sleep(time.Millisecond * 10)
	gcCache(cache)
	assert.Equal(t, cache.Get("0").Value(), 0)
	assert.Equal(t, cache.Get("1").Value(), 1)
	item0.Release()
	item1.Release()
	gcCache(cache)
	assert.Nil(t, cache.Get("0"))
	assert.Nil(t, cache.Get("1"))
}

func Test_Cache_RemovesOldestItemWhenFull(t *testing.T) {
	onDeleteFnCalled := false
	onDeleteFn := func(item *Item) {
		if item.key == "0" {
			onDeleteFnCalled = true
		}
	}

	cache := New(Configure().MaxSize(5).ItemsToPrune(1).OnDelete(onDeleteFn))
	for i := 0; i < 7; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	time.Sleep(time.Millisecond * 10)
	assert.Nil(t, cache.Get("0"))
	assert.Nil(t, cache.Get("1"))
	assert.Equal(t, cache.Get("2").Value(), 2)
	assert.Equal(t, onDeleteFnCalled, true)
	assert.Equal(t, cache.ItemCount(), 5)
}

func Test_Cache_RemovesOldestItemWhenFullBySizer(t *testing.T) {
	cache := New(Configure().MaxSize(9).ItemsToPrune(2))
	for i := 0; i < 7; i++ {
		cache.Set(strconv.Itoa(i), &SizedItem{i, 2}, time.Minute)
	}
	time.Sleep(time.Millisecond * 10)
	assert.Nil(t, cache.Get("0"))
	assert.Nil(t, cache.Get("1"))
	assert.Nil(t, cache.Get("2"))
	assert.Nil(t, cache.Get("3"))
	assert.Equal(t, cache.Get("4").Value().(*SizedItem).id, 4)
	assert.Equal(t, cache.GetDropped(), 4)
	assert.Equal(t, cache.GetDropped(), 0)
}

func Test_Cache_SetUpdatesSizeOnDelta(t *testing.T) {
	cache := New(Configure())
	cache.Set("a", &SizedItem{0, 2}, time.Minute)
	cache.Set("b", &SizedItem{0, 3}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 5)
	cache.Set("b", &SizedItem{0, 3}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 5)
	cache.Set("b", &SizedItem{0, 4}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 6)
	cache.Set("b", &SizedItem{0, 2}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 4)
	cache.Delete("b")
	time.Sleep(time.Millisecond * 100)
	checkSize(t, cache, 2)
}

func Test_Cache_ReplaceDoesNotchangeSizeIfNotSet(t *testing.T) {
	cache := New(Configure())
	cache.Set("1", &SizedItem{1, 2}, time.Minute)
	cache.Set("2", &SizedItem{1, 2}, time.Minute)
	cache.Set("3", &SizedItem{1, 2}, time.Minute)
	cache.Replace("4", &SizedItem{1, 2})
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 6)
}

func Test_Cache_ReplaceChangesSize(t *testing.T) {
	cache := New(Configure())
	cache.Set("1", &SizedItem{1, 2}, time.Minute)
	cache.Set("2", &SizedItem{1, 2}, time.Minute)

	cache.Replace("2", &SizedItem{1, 2})
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 4)

	cache.Replace("2", &SizedItem{1, 1})
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 3)

	cache.Replace("2", &SizedItem{1, 3})
	time.Sleep(time.Millisecond * 5)
	checkSize(t, cache, 5)
}

func Test_Cache_ResizeOnTheFly(t *testing.T) {
	// On a busy system or during a slow run, the cleanup might take longer.
	// When this happens, the test continues
	// and runs its assertions (e.g., assert.Equal(t, cache.GetDropped(), 2))
	// before the cache has actually been pruned, causing the test to fail.
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	cache := New(Configure().MaxSize(9).ItemsToPrune(1))
	for i := 0; i < 5; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SetMaxSize(3)
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, cache.GetDropped(), 2)
	assert.Nil(t, cache.Get("0"))
	assert.Nil(t, cache.Get("1"))
	assert.Equal(t, cache.Get("2").Value(), 2)
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)

	cache.Set("5", 5, time.Minute)
	time.Sleep(time.Millisecond * 5)
	assert.Equal(t, cache.GetDropped(), 1)
	assert.Nil(t, cache.Get("2"))
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)
	assert.Equal(t, cache.Get("5").Value(), 5)

	cache.SetMaxSize(10)
	cache.Set("6", 6, time.Minute)
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, cache.GetDropped(), 0)
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)
	assert.Equal(t, cache.Get("5").Value(), 5)
	assert.Equal(t, cache.Get("6").Value(), 6)
}

type SizedItem struct {
	id int
	s  int64
}

func (s *SizedItem) Size() int64 {
	return s.s
}

func checkSize(t *testing.T, cache *Cache, sz int64) {
	cache.Stop()
	assert.Equal(t, cache.size, sz)
	cache.restart()
}

func gcCache(cache *Cache) {
	cache.Stop()
	cache.gc()
	cache.restart()
}
