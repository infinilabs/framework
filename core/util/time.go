/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package util

import (
	"encoding/binary"
	"github.com/segmentio/encoding/json"
	"errors"
	"hash"
	"time"
)

func MaxDuration(d1 time.Duration, d2 time.Duration) time.Duration {
	if d1 > d2 {
		return d1
	} else {
		return d2
	}
}

func MinDuration(d1 time.Duration, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	} else {
		return d2
	}
}


// TsLayout is the layout to be used in the timestamp marshaling/unmarshaling everywhere.
// The timezone must always be UTC.
const TsLayout = "2006-01-02T15:04:05.000Z"

// Time is an abstraction for the time.Time type
type Time time.Time

// MarshalJSON implements json.Marshaler interface.
// The time is a quoted string in the JsTsLayout format.
func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).UTC().Format(TsLayout))
}

// UnmarshalJSON implements js.Unmarshaler interface.
// The time is expected to be a quoted string in TsLayout
// format.
func (t *Time) UnmarshalJSON(data []byte) (err error) {
	if data[0] != []byte(`"`)[0] || data[len(data)-1] != []byte(`"`)[0] {
		return errors.New("Not quoted")
	}
	*t, err = ParseTimeWithStandardSpec(string(data[1 : len(data)-1]))
	return
}

func (t Time) Hash32(h hash.Hash32) error {
	err := binary.Write(h, binary.LittleEndian, time.Time(t).UnixNano())
	return err
}

// ParseTime parses a time in the TsLayout format.
func ParseTimeWithStandardSpec(value string) (Time, error) {
	t, err := time.Parse(TsLayout, value)
	return Time(t), err
}

func ParseStandardTime(value string) (time.Time, error) {
	t, err := time.Parse(TsLayout, value)
	return t, err
}


func (t Time) String() string {
	return time.Time(t).Format(TsLayout)
}

// MustParseTime is a convenience equivalent of the ParseTime function
// that panics in case of errors.
func MustParseTime(value string) Time {
	ts, err := ParseTimeWithStandardSpec(value)
	if err != nil {
		panic(err)
	}

	return ts
}


//old

func FormatTime(date time.Time) string {
	return date.Format("2006-01-02 15:04:05")
}

func ParseTime(str string) time.Time  {
	v,err:= time.Parse("2006-01-02 15:04:05",str)
	if err!=nil{
		panic(err)
	}
	return v
}

func FormatTimeForFileName(date time.Time) string {
	return date.Format("2006-01-02_150405")
}

func FormatUnixTimestamp(unix int64) string {
	date := FromUnixTimestamp(unix)
	return date.Format("2006-01-02 15:04:05")
}
func FromUnixTimestamp(unix int64) time.Time {
	return time.Unix(unix, 0)
}

func FormatTimeWithLocalTZ(date time.Time) string {
	localLoc, err := time.LoadLocation("Local")
	if err != nil {
		panic(errors.New(`Failed to load location "Local"`))
	}
	localDateTime := date.In(localLoc)

	return localDateTime.Format("2006-01-02 15:04:05")
}

func FormatTimeWithTZ(date time.Time) string {
	return date.Format("2016-10-24 09:34:19 +0000 UTC")
}

// GetLocalZone return a local timezone
func GetLocalZone() string {
	zone, _ := time.Now().Zone()
	return zone
}
