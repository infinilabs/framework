module infini.sh/framework

go 1.23.3

replace github.com/libdns/libdns => ../vendor/src/github.com/libdns/libdns

replace github.com/libdns/tencentcloud => ../vendor/src/github.com/libdns/tencentcloud

replace github.com/caddyserver/certmagic => ../vendor/src/github.com/caddyserver/certmagic

replace github.com/caddyserver/zerossl => ../vendor/src/github.com/caddyserver/zerossl

replace github.com/quipo/statsd => ../vendor/src/github.com/quipo/statsd

replace github.com/cihub/seelog => ../vendor/src/github.com/cihub/seelog

replace github.com/gopkg.in/gomail.v2 => ../vendor/src/github.com/gopkg.in/gomail.v2

require (
	github.com/OneOfOne/xxhash v1.2.8
	github.com/RoaringBitmap/roaring v1.9.4
	github.com/andybalholm/brotli v1.1.1
	github.com/arl/statsviz v0.6.0
	github.com/bkaradzic/go-lz4 v1.0.0
	github.com/buger/jsonparser v1.1.1
	github.com/caddyserver/certmagic v0.23.0
	github.com/cihub/seelog v0.0.0-00010101000000-000000000000
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/dgraph-io/badger/v4 v4.7.0
	github.com/dgraph-io/ristretto v0.2.0
	github.com/emirpasic/gods v1.18.1
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-ldap/ldap/v3 v3.4.11
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/google/go-cmp v0.7.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/gopkg.in/gomail.v2 v0.0.0-00010101000000-000000000000
	github.com/gorilla/context v1.1.2
	github.com/gorilla/sessions v1.4.0
	github.com/gorilla/websocket v1.5.3
	github.com/hashicorp/go-version v1.7.0
	github.com/jmoiron/jsonq v0.0.0-20150511023944-e874b168d07e
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/kardianos/service v1.2.2
	github.com/klauspost/compress v1.18.0
	github.com/libdns/tencentcloud v1.2.1
	github.com/magiconair/properties v1.8.10
	github.com/mailru/easyjson v0.9.0
	github.com/minio/minio-go/v7 v7.0.90
	github.com/nsqio/nsq v1.3.0
	github.com/pkg/errors v0.9.1
	github.com/quipo/statsd v0.0.0-00010101000000-000000000000
	github.com/r3labs/diff/v2 v2.15.1
	github.com/rs/cors v1.11.1
	github.com/rs/xid v1.6.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/savsgio/gotils v0.0.0-20250408102913-196191ec6287
	github.com/segmentio/encoding v0.4.1
	github.com/seiflotfy/cuckoofilter v0.0.0-20240715131351-a2f2c23f1771
	github.com/shaj13/go-guardian/v2 v2.11.6
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	github.com/twmb/franz-go v1.18.1
	github.com/twmb/franz-go/pkg/kadm v1.16.0
	github.com/twmb/franz-go/pkg/kmsg v1.11.2
	github.com/valyala/fasttemplate v1.2.2
	github.com/valyala/tcplisten v1.0.0
	github.com/zeebo/sbloom v0.0.0-20151106181526-405c65bd9be0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.37.0
	golang.org/x/net v0.39.0
	golang.org/x/oauth2 v0.29.0
	golang.org/x/sys v0.32.0
	golang.org/x/text v0.24.0
	golang.org/x/time v0.11.0
	golang.org/x/tools v0.32.0
	google.golang.org/grpc v1.71.1
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/hjson/hjson-go.v3 v3.3.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v2 v2.4.0
	infini.sh/license v0.0.0-00010101000000-000000000000
	k8s.io/api v0.32.3
	k8s.io/apimachinery v0.32.3
)

require (
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/bits-and-blooms/bitset v1.12.0 // indirect
	github.com/caddyserver/zerossl v0.1.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgraph-io/ristretto/v2 v2.2.0 // indirect
	github.com/dgryski/go-metro v0.0.0-20200812162917-85c65e2d0165 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.8-0.20250403174932-29230038a667 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/libdns/libdns v1.0.0-beta.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mholt/acmez/v2 v2.0.3 // indirect
	github.com/miekg/dns v1.1.63 // indirect
	github.com/minio/crc64nvme v1.0.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.1051 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod v1.0.1033 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
