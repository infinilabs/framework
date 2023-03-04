package ccache

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_SecondaryCache_GetsANonExistantValue(t *testing.T) {
	cache := newLayered().GetOrCreateSecondaryCache("foo")
	assert.NotNil(t, cache)
}

func Test_SecondaryCache_SetANewValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "a value", time.Minute)
	sCache := cache.GetOrCreateSecondaryCache("spice")
	assert.Equal(t, sCache.Get("flow").Value(), "a value")
	assert.Nil(t, sCache.Get("stop"))
}

func Test_SecondaryCache_ValueCanBeSeenInBothCaches1(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "a value", time.Minute)
	sCache := cache.GetOrCreateSecondaryCache("spice")
	sCache.Set("orinoco", "another value", time.Minute)
	assert.Equal(t, sCache.Get("orinoco").Value(), "another value")
	assert.Equal(t, cache.Get("spice", "orinoco").Value(), "another value")
}

func Test_SecondaryCache_ValueCanBeSeenInBothCaches2(t *testing.T) {
	cache := newLayered()
	sCache := cache.GetOrCreateSecondaryCache("spice")
	sCache.Set("flow", "a value", time.Minute)
	assert.Equal(t, sCache.Get("flow").Value(), "a value")
	assert.Equal(t, cache.Get("spice", "flow").Value(), "a value")
}

func Test_SecondaryCache_DeletesAreReflectedInBothCaches(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "a value", time.Minute)
	cache.Set("spice", "sister", "ghanima", time.Minute)
	sCache := cache.GetOrCreateSecondaryCache("spice")

	cache.Delete("spice", "flow")
	assert.Nil(t, cache.Get("spice", "flow"))
	assert.Nil(t, sCache.Get("flow"))

	sCache.Delete("sister")
	assert.Nil(t, cache.Get("spice", "sister"))
	assert.Nil(t, sCache.Get("sister"))
}

func Test_SecondaryCache_ReplaceDoesNothingIfKeyDoesNotExist(t *testing.T) {
	cache := newLayered()
	sCache := cache.GetOrCreateSecondaryCache("spice")
	assert.Equal(t, sCache.Replace("flow", "value-a"), false)
	assert.Nil(t, cache.Get("spice", "flow"))
}

func Test_SecondaryCache_ReplaceUpdatesTheValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	sCache := cache.GetOrCreateSecondaryCache("spice")
	assert.Equal(t, sCache.Replace("flow", "value-b"), true)
	assert.Equal(t, cache.Get("spice", "flow").Value().(string), "value-b")
}

func Test_SecondaryCache_FetchReturnsAnExistingValue(t *testing.T) {
	cache := newLayered()
	cache.Set("spice", "flow", "value-a", time.Minute)
	sCache := cache.GetOrCreateSecondaryCache("spice")
	val, _ := sCache.Fetch("flow", time.Minute, func() (interface{}, error) { return "a fetched value", nil })
	assert.Equal(t, val.Value().(string), "value-a")
}

func Test_SecondaryCache_FetchReturnsANewValue(t *testing.T) {
	cache := newLayered()
	sCache := cache.GetOrCreateSecondaryCache("spice")
	val, _ := sCache.Fetch("flow", time.Minute, func() (interface{}, error) { return "a fetched value", nil })
	assert.Equal(t, val.Value().(string), "a fetched value")
}

func Test_SecondaryCache_TrackerDoesNotCleanupHeldInstance(t *testing.T) {
	cache := Layered(Configure().ItemsToPrune(10).Track())
	for i := 0; i < 10; i++ {
		cache.Set(strconv.Itoa(i), "a", i, time.Minute)
	}
	sCache := cache.GetOrCreateSecondaryCache("0")
	item := sCache.TrackingGet("a")
	time.Sleep(time.Millisecond * 10)
	gcLayeredCache(cache)
	assert.Equal(t, cache.Get("0", "a").Value(), 0)
	assert.Nil(t, cache.Get("1", "a"))
	item.Release()
	gcLayeredCache(cache)
	assert.Nil(t, cache.Get("0", "a"))
}
