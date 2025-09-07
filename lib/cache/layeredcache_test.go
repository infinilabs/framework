package ccache

import (
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_LayeredCache_GetsANonExistantValue(t *testing.T) {
	cache := newLayered()
	assert.Nil(t, cache.Get("spice", "flow"))
	assert.Equal(t, cache.ItemCount(), 0)
}

func Test_LayeredCache_SetANewValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "a value", time.Minute)
	assert.Equal(t, cache.Get("spice", "flow").Value(), "a value")
	assert.Nil(t, cache.Get("spice", "stop"))
	assert.Equal(t, cache.ItemCount(), 1)
}

func Test_LayeredCache_SetsMultipleValueWithinTheSameLayer(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	cache.Set("spice", "must", "value-b", time.Minute)
	cache.Set("leto", "sister", "ghanima", time.Minute)
	assert.Equal(t, cache.Get("spice", "flow").Value(), "value-a")
	assert.Equal(t, cache.Get("spice", "must").Value(), "value-b")
	assert.Nil(t, cache.Get("spice", "worm"))

	assert.Equal(t, cache.Get("leto", "sister").Value(), "ghanima")
	assert.Nil(t, cache.Get("leto", "brother"))
	assert.Nil(t, cache.Get("baron", "friend"))
	assert.Equal(t, cache.ItemCount(), 3)
}

func Test_LayeredCache_ReplaceDoesNothingIfKeyDoesNotExist(t *testing.T) {
	cache := newLayered()
	assert.Equal(t, cache.Replace("spice", "flow", "value-a"), false)
	assert.Nil(t, cache.Get("spice", "flow"))
}

func Test_LayeredCache_ReplaceUpdatesTheValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	assert.Equal(t, cache.Replace("spice", "flow", "value-b"), true)
	assert.Equal(t, cache.Get("spice", "flow").Value().(string), "value-b")
	assert.Equal(t, cache.ItemCount(), 1)
	//not sure how to test that the TTL hasn't changed sort of a sleep..
}

func Test_LayeredCache_DeletesAValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	cache.Set("spice", "must", "value-b", time.Minute)
	cache.Set("leto", "sister", "ghanima", time.Minute)
	cache.Delete("spice", "flow")
	assert.Nil(t, cache.Get("spice", "flow"))
	assert.Equal(t, cache.Get("spice", "must").Value(), "value-b")
	assert.Nil(t, cache.Get("spice", "worm"))
	assert.Equal(t, cache.Get("leto", "sister").Value(), "ghanima")
	assert.Equal(t, cache.ItemCount(), 2)
}

func Test_LayeredCache_DeletesAPrefix(t *testing.T) {
	cache := newLayered()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("spice", "aaa", "1", time.Minute)
	cache.Set("spice", "aab", "2", time.Minute)
	cache.Set("spice", "aac", "3", time.Minute)
	cache.Set("leto", "aac", "3", time.Minute)
	cache.Set("spice", "ac", "4", time.Minute)
	cache.Set("spice", "z5", "7", time.Minute)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeletePrefix("spice", "9a"), 0)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeletePrefix("spice", "aa"), 3)
	assert.Nil(t, cache.Get("spice", "aaa"))
	assert.Nil(t, cache.Get("spice", "aab"))
	assert.Nil(t, cache.Get("spice", "aac"))
	assert.Equal(t, cache.Get("spice", "ac").Value(), "4")
	assert.Equal(t, cache.Get("spice", "z5").Value(), "7")
	assert.Equal(t, cache.ItemCount(), 3)
}

func Test_LayeredCache_DeletesAFunc(t *testing.T) {
	cache := newLayered()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("spice", "a", 1, time.Minute)
	cache.Set("leto", "b", 2, time.Minute)
	cache.Set("spice", "c", 3, time.Minute)
	cache.Set("spice", "d", 4, time.Minute)
	cache.Set("spice", "e", 5, time.Minute)
	cache.Set("spice", "f", 6, time.Minute)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc("spice", func(key string, item *Item) bool {
		return false
	}), 0)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc("spice", func(key string, item *Item) bool {
		return item.Value().(int) < 4
	}), 2)
	assert.Equal(t, cache.ItemCount(), 4)

	assert.Equal(t, cache.DeleteFunc("spice", func(key string, item *Item) bool {
		return key == "d"
	}), 1)
	assert.Equal(t, cache.ItemCount(), 3)

}

func Test_LayeredCache_OnDeleteCallbackCalled(t *testing.T) {
	onDeleteFnCalled := int32(0)
	onDeleteFn := func(item *Item) {
		if item.group == "spice" && item.key == "flow" {
			atomic.AddInt32(&onDeleteFnCalled, 1)
		}
	}

	cache := Layered(Configure().OnDelete(onDeleteFn))
	cache.Set("spice", "flow", "value-a", time.Minute)
	cache.Set("spice", "must", "value-b", time.Minute)
	cache.Set("leto", "sister", "ghanima", time.Minute)

	time.Sleep(time.Millisecond * 10) // Run once to init
	cache.Delete("spice", "flow")
	time.Sleep(time.Millisecond * 10) // Wait for worker to pick up deleted items

	assert.Nil(t, cache.Get("spice", "flow"))
	assert.Equal(t, cache.Get("spice", "must").Value(), "value-b")
	assert.Nil(t, cache.Get("spice", "worm"))
	assert.Equal(t, cache.Get("leto", "sister").Value(), "ghanima")

	assert.Equal(t, atomic.LoadInt32(&onDeleteFnCalled), int32(1))
}

func Test_LayeredCache_DeletesALayer(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	cache.Set("spice", "must", "value-b", time.Minute)
	cache.Set("leto", "sister", "ghanima", time.Minute)
	cache.DeleteAll("spice")
	assert.Nil(t, cache.Get("spice", "flow"))
	assert.Nil(t, cache.Get("spice", "must"))
	assert.Nil(t, cache.Get("spice", "worm"))
	assert.Equal(t, cache.Get("leto", "sister").Value(), "ghanima")
}

func Test_LayeredCache_GCsTheOldestItems(t *testing.T) {
	cache := Layered(Configure().ItemsToPrune(10))
	cache.Set("xx", "a", 23, time.Minute)
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	cache.Set("xx", "b", 9001, time.Minute)
	//let the items get promoted (and added to our list)
	time.Sleep(time.Millisecond * 10)
	gcLayeredCache(cache)
	assert.Nil(t, cache.Get("xx", "a"))
	assert.Equal(t, cache.Get("xx", "b").Value(), 9001)
	assert.Nil(t, cache.Get("8", "a"))
	assert.Equal(t, cache.Get("9", "a").Value(), 9)
	assert.Equal(t, cache.Get("10", "a").Value(), 10)
}

func Test_LayeredCache_PromotedItemsDontGetPruned(t *testing.T) {
	cache := Layered(Configure().ItemsToPrune(10).GetsPerPromote(1))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	time.Sleep(time.Millisecond * 10) //run the worker once to init the list
	cache.Get("9", "a")
	time.Sleep(time.Millisecond * 10)
	gcLayeredCache(cache)
	assert.Equal(t, cache.Get("9", "a").Value(), 9)
	assert.Nil(t, cache.Get("10", "a"))
	assert.Equal(t, cache.Get("11", "a").Value(), 11)
}

func Test_LayeredCache_TrackerDoesNotCleanupHeldInstance(t *testing.T) {
	cache := Layered(Configure().ItemsToPrune(10).Track())
	item0 := cache.TrackingSet("0", "a", 0, time.Minute)
	for i := 1; i < 11; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	item1 := cache.TrackingGet("1", "a")
	time.Sleep(time.Millisecond * 10)
	gcLayeredCache(cache)
	assert.Equal(t, cache.Get("0", "a").Value(), 0)
	assert.Equal(t, cache.Get("1", "a").Value(), 1)
	item0.Release()
	item1.Release()
	gcLayeredCache(cache)
	assert.Nil(t, cache.Get("0", "a"))
	assert.Nil(t, cache.Get("1", "a"))
}

func Test_LayeredCache_RemovesOldestItemWhenFull(t *testing.T) {
	cache := Layered(Configure().MaxSize(5).ItemsToPrune(1))
	cache.Set("xx", "a", 23, time.Minute)
	for i := 0; i < 7; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	cache.Set("xx", "b", 9001, time.Minute)
	time.Sleep(time.Millisecond * 10)
	assert.Nil(t, cache.Get("xx", "a"))
	assert.Nil(t, cache.Get("0", "a"))
	assert.Nil(t, cache.Get("1", "a"))
	assert.Nil(t, cache.Get("2", "a"))
	assert.Equal(t, cache.Get("3", "a").Value(), 3)
	assert.Equal(t, cache.Get("xx", "b").Value(), 9001)
	assert.Equal(t, cache.GetDropped(), 4)
	assert.Equal(t, cache.GetDropped(), 0)
}

func Test_LayeredCache_ResizeOnTheFly(t *testing.T) {
	// On a busy system or during a slow run, the cleanup might take longer.
	// When this happens, the test continues
	// and runs its assertions (e.g., assert.Equal(t, cache.GetDropped(), 2))
	// before the cache has actually been pruned, causing the test to fail.
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	cache := Layered(Configure().MaxSize(9).ItemsToPrune(1))
	for i := 0; i < 5; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	cache.SetMaxSize(3)
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, cache.GetDropped(), 2)
	assert.Nil(t, cache.Get("0", "a"))
	assert.Nil(t, cache.Get("1", "a"))
	assert.Equal(t, cache.Get("2", "a").Value(), 2)
	assert.Equal(t, cache.Get("3", "a").Value(), 3)
	assert.Equal(t, cache.Get("4", "a").Value(), 4)

	cache.Set("5", "a", 5, time.Minute)
	time.Sleep(time.Millisecond * 5)
	assert.Equal(t, cache.GetDropped(), 1)
	assert.Nil(t, cache.Get("2", "a"))
	assert.Equal(t, cache.Get("3", "a").Value(), 3)
	assert.Equal(t, cache.Get("4", "a").Value(), 4)
	assert.Equal(t, cache.Get("5", "a").Value(), 5)

	cache.SetMaxSize(10)
	cache.Set("6", "a", 6, time.Minute)
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, cache.GetDropped(), 0)
	assert.Equal(t, cache.Get("3", "a").Value(), 3)
	assert.Equal(t, cache.Get("4", "a").Value(), 4)
	assert.Equal(t, cache.Get("5", "a").Value(), 5)
	assert.Equal(t, cache.Get("6", "a").Value(), 6)
}

func newLayered() *LayeredCache {
	c := Layered(Configure())
	c.Clear()
	return c
}

func Test_LayeredCache_RemovesOldestItemWhenFullBySizer(t *testing.T) {
	cache := Layered(Configure().MaxSize(9).ItemsToPrune(2))
	for i := 0; i < 7; i++ {
		cache.Set("pri", strconv.Itoa(i), &SizedItem{i, 2}, time.Minute)
	}
	time.Sleep(time.Millisecond * 10)
	assert.Nil(t, cache.Get("pri", "0"))
	assert.Nil(t, cache.Get("pri", "1"))
	assert.Nil(t, cache.Get("pri", "2"))
	assert.Nil(t, cache.Get("pri", "3"))
	assert.Equal(t, cache.Get("pri", "4").Value().(*SizedItem).id, 4)
}

func Test_LayeredCache_SetUpdatesSizeOnDelta(t *testing.T) {
	cache := Layered(Configure())
	cache.Set("pri", "a", &SizedItem{0, 2}, time.Minute)
	cache.Set("pri", "b", &SizedItem{0, 3}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 5)
	cache.Set("pri", "b", &SizedItem{0, 3}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 5)
	cache.Set("pri", "b", &SizedItem{0, 4}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 6)
	cache.Set("pri", "b", &SizedItem{0, 2}, time.Minute)
	cache.Set("sec", "b", &SizedItem{0, 3}, time.Minute)
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 7)
	cache.Delete("pri", "b")
	time.Sleep(time.Millisecond * 10)
	checkLayeredSize(t, cache, 5)
}

func Test_LayeredCache_ReplaceDoesNotchangeSizeIfNotSet(t *testing.T) {
	cache := Layered(Configure())
	cache.Set("pri", "1", &SizedItem{1, 2}, time.Minute)
	cache.Set("pri", "2", &SizedItem{1, 2}, time.Minute)
	cache.Set("pri", "3", &SizedItem{1, 2}, time.Minute)
	cache.Replace("sec", "3", &SizedItem{1, 2})
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 6)
}

func Test_LayeredCache_ReplaceChangesSize(t *testing.T) {
	cache := Layered(Configure())
	cache.Set("pri", "1", &SizedItem{1, 2}, time.Minute)
	cache.Set("pri", "2", &SizedItem{1, 2}, time.Minute)

	cache.Replace("pri", "2", &SizedItem{1, 2})
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 4)

	cache.Replace("pri", "2", &SizedItem{1, 1})
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 3)

	cache.Replace("pri", "2", &SizedItem{1, 3})
	time.Sleep(time.Millisecond * 5)
	checkLayeredSize(t, cache, 5)
}

func checkLayeredSize(t *testing.T, cache *LayeredCache, sz int64) {
	cache.Stop()
	assert.Equal(t, cache.size, sz)
	cache.restart()
}

func gcLayeredCache(cache *LayeredCache) {
	cache.Stop()
	cache.gc()
	cache.restart()
}
