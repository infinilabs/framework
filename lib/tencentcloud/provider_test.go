package tencentcloud

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/libdns/libdns"
)

var provider = &Provider{
	SecretId:  os.Getenv("TC_SECRET_ID"),
	SecretKey: os.Getenv("TC_SECRET_KEY"),
}

var (
	zone  = os.Getenv("TC_ZONE")
	name  = os.Getenv("TC_NAME")
	value = os.Getenv("TC_VALUE")
)

func TestSetRecords(t *testing.T) {
	netip, err := netip.ParseAddr(value)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = provider.SetRecords(context.Background(), zone, []libdns.Record{
		libdns.Address{
			Name: name,
			IP:   netip,
		},
	})
	if err != nil {
		t.Fatalf("SetRecords: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	records, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	for _, record := range records {
		rr := record.RR()
		t.Logf("RecordType: %s, Name: %s, Data: %s",
			rr.Type, rr.Name, rr.Data)
	}
}
