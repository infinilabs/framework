package ccache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Bucket_GetMissFromBucket(t *testing.T) {
	bucket := testBucket()
	assert.Nil(t, bucket.get("invalid"))
}

func Test_Bucket_GetHitFromBucket(t *testing.T) {
	bucket := testBucket()
	item := bucket.get("power")
	assert.Equal(t, item.value, TestValue("9000"))
}

func Test_Bucket_DeleteItemFromBucket(t *testing.T) {
	bucket := testBucket()
	bucket.delete("power")
	assert.Nil(t, bucket.get("power"))
}

func Test_Bucket_SetsANewBucketItem(t *testing.T) {
	bucket := testBucket()
	item, existing := bucket.set("spice", TestValue("flow"), time.Minute, false)
	assert.Equal(t, item.value, TestValue("flow"))
	item = bucket.get("spice")
	assert.Equal(t, item.value, TestValue("flow"))
	assert.Nil(t, existing)
}

func Test_Bucket_SetsAnExistingItem(t *testing.T) {
	bucket := testBucket()
	item, existing := bucket.set("power", TestValue("9001"), time.Minute, false)
	assert.Equal(t, item.value, TestValue("9001"))
	item = bucket.get("power")
	assert.Equal(t, item.value, TestValue("9001"))
	assert.Equal(t, existing.value, TestValue("9000"))
}

func testBucket() *bucket {
	b := &bucket{lookup: make(map[string]*Item)}
	b.lookup["power"] = &Item{
		key:   "power",
		value: TestValue("9000"),
	}
	return b
}

type TestValue string

func (v TestValue) Expires() time.Time {
	return time.Now()
}
