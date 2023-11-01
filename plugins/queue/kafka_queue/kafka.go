/* Copyright © INFINI Ltd. All rights reserved.
* web: https://infinilabs.com
* mail: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"crypto/tls"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/module"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"net"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"sync"
	"time"
)

func init() {
	module.RegisterSystemModule(&KafkaQueue{})
}

type Config struct {
	Enabled               bool   `config:"enabled"`
	Default               bool   `config:"default"`
	Compression           bool   `config:"compression"`
	NumOfPartition        int    `config:"num_of_partition"`
	NumOfReplica          int    `config:"num_of_replica"`
	Prefix                string `config:"prefix"`
	ProducerBatchMaxBytes int32  `config:"producer_batch_max_bytes"`
	MaxBufferedRecords    int    `config:"max_buffered_records"`
	ManualFlushing        bool   `config:"manual_flushing"`

	Brokers  []string `config:"brokers"`
	Username string   `config:"username"`
	Password string   `config:"password"`
	TLS      bool     `config:"tls"`

	Mechanism string `config:"mechanism"`
}

//const PLAIN_MECHANISM = "PLAIN"//
const SCRAM_SHA_256_Mechanism = "SCRAM-SHA-256"
const SCRAM_SHA_512_Mechanism = "SCRAM-SHA-512"

type KafkaQueue struct {
	cfg         *Config
	q           sync.Map
	consumers   sync.Map //q+consumer=instance
	producers   sync.Map //q=instance
	adminClient *kadm.Client
}

func (this *KafkaQueue) newClient(opt []kgo.Opt) *kgo.Client {
	opts := []kgo.Opt{
		kgo.SeedBrokers(this.cfg.Brokers...),
	}

	if !this.cfg.Compression {
		opts = append(opts, kgo.ProducerBatchCompression(kgo.NoCompression()))
	}

	if this.cfg.Username != "" {
		if this.cfg.TLS{
			tlsDialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}}
			opts = append(opts, kgo.Dialer(tlsDialer.DialContext))
		}

		// SASL Options
		switch this.cfg.Mechanism {
		case SCRAM_SHA_256_Mechanism:
			opts = append(opts,
				kgo.SASL(scram.Auth{
					User: this.cfg.Username,
					Pass: this.cfg.Password,
				}.AsSha256Mechanism()),
			)
			break
		case SCRAM_SHA_512_Mechanism:
			opts = append(opts,
				kgo.SASL(scram.Auth{
					User: this.cfg.Username,
					Pass: this.cfg.Password,
				}.AsSha512Mechanism()),
			)
			break
		default:
			opts = append(opts,
				kgo.SASL(plain.Auth{
					User: this.cfg.Username,
					Pass: this.cfg.Password,
				}.AsMechanism()))
		}
	}

	if opt != nil {
		opts = append(opts, opt...)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		panic(err)
	}
	return client
}

func getGroupForKafka(group, topic string) string {
	return group + "_" + topic
}

func (this *KafkaQueue) ReleaseConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig, instance queue.ConsumerAPI) error {
	this.consumers.Delete(getGroupForKafka(consumer.Group, qconfig.ID))
	k := getGroupForKafka(consumer.Group, qconfig.ID)
	err := locker.Release(queue.BucketWhoOwnsThisTopic, k, global.Env().SystemConfig.NodeConfig.ID)
	if err != nil {
		return err
	}
	if instance != nil {
		return instance.Close()
	}
	return nil
}

func (this *KafkaQueue) AcquireConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig) (queue.ConsumerAPI, error) {

	//check if consumer is already acquired for queue
	//if yes, whois the owner
	//if yes, how long it takes, is it already expired
	//if no, acquire it
	//place a lock on it
	//periodically update last access time

	var err error
	_, ok := this.q.Load(qconfig.ID)
	if !ok {
		//try init
		err = this.Init(qconfig.ID)
		if err != nil {
			panic(err)
		}
		_, ok = this.q.Load(qconfig.ID)
	}

	if ok {

		k := getGroupForKafka(consumer.Group, qconfig.ID)
		ok, err := locker.Hold(queue.BucketWhoOwnsThisTopic, k, global.Env().SystemConfig.NodeConfig.ID, 60*time.Second, true)
		if !ok || err != nil {
			panic(errors.Errorf("allocate consumer failed, consumer is already acquired by another node, key:%v err: %v", string(k), err))
		}

		opts := []kgo.Opt{
			kgo.InstanceID(consumer.ID),
			kgo.SessionTimeout(30 * time.Second),
			kgo.HeartbeatInterval(10 * time.Second),
			kgo.RetryTimeout(10 * time.Second),
			kgo.RebalanceTimeout(10 * time.Second),
			kgo.TransactionTimeout(10 * time.Second),
			kgo.RequestTimeoutOverhead(10 * time.Second),
			kgo.ClientID(global.Env().SystemConfig.NodeConfig.ID),
			kgo.ConsumerGroup(getGroupForKafka(consumer.Group, qconfig.ID)),
			kgo.ConsumeTopics(qconfig.ID),
			kgo.AllowAutoTopicCreation(),
			kgo.FetchMinBytes(int32(consumer.FetchMinBytes)),
			kgo.FetchMaxBytes(int32(consumer.FetchMaxBytes)),
			kgo.FetchMaxWait(time.Duration(consumer.FetchMaxWaitMs) * time.Millisecond),
		}

		if consumer.AutoResetOffset == "earliest" {
			opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		} else if consumer.AutoResetOffset == "latest" {
			opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
		}

		if !consumer.AutoCommitOffset {
			opts = append(opts, kgo.DisableAutoCommit())
		}

		client := this.newClient(opts)

		output := Consumer{
			qCfg:   qconfig,
			cCfg:   consumer,
			client: client,
		}

		this.consumers.Store(getGroupForKafka(consumer.Group, qconfig.ID), &output)
		if global.Env().IsDebug {
			log.Infof("acquired consumer:%v, %v, %v", qconfig.Name, consumer.Key(), consumer.ID)
		}

		return &output, err
	}
	panic(errors.Errorf("queue [%v] not found", qconfig.Name))
}

func (this *KafkaQueue) Destroy(k string) error {
	_, err := this.adminClient.DeleteTopics(context.Background(), k)
	if err != nil {
		panic(err)
	}
	this.q.Delete(k)
	return nil
}

func (this *KafkaQueue) Setup() {
	this.q = sync.Map{}
	this.consumers = sync.Map{}
	this.producers = sync.Map{}
	this.cfg = &Config{
		Enabled:               false,
		Compression:           false,
		TLS:                   true,
		NumOfPartition:        1,
		ProducerBatchMaxBytes: 50 * 1024 * 1024,
		MaxBufferedRecords:    10000,
		NumOfReplica:          1,
	}

	ok, err := env.ParseConfig("kafka_queue", this.cfg)
	if ok && err != nil  &&global.Env().SystemConfig.Configs.PanicOnConfigError{
		panic(err)
	}

	if !this.cfg.Enabled {
		return
	}

	client := this.newClient(nil)
	this.adminClient = kadm.NewClient(client)

	queue.Register("kafka", this)

	if this.cfg.Default {
		queue.RegisterDefaultHandler(this)
	}

}

func (this *KafkaQueue) Start() error {
	if this.cfg != nil && !this.cfg.Enabled {
		return nil
	}
	return nil
}

func (this *KafkaQueue) Stop() error {
	if this.cfg == nil {
		return nil
	}

	if this.cfg != nil && this.cfg.Enabled {
		this.adminClient.Close()
	}

	return nil
}

func (this *KafkaQueue) Name() string {
	return "kafka_queue"
}

func (this *KafkaQueue) getRealTopicName(k string) string {
	if this.cfg.Prefix != "" {
		return this.cfg.Prefix + k
	}
	return k
}

func (this *KafkaQueue) createTopic(topic string, partition, replica int) error {
	res, err := this.adminClient.CreateTopic(context.Background(), int32(partition), int16(replica), nil, topic)
	log.Tracef("create topic:%v, %v, %v, %v,%v", topic, partition, replica, res, err)
	return err
}

func (this *KafkaQueue) Init(q string) error {
	if c, ok := this.q.Load(q); ok && c != nil {
		client, ok := c.(*kgo.Client)
		if ok && client != nil {
			return nil
		}
	}
	opts := []kgo.Opt{
		kgo.ConsumeTopics(q),
	}
	client := this.newClient(opts)
	err := this.createTopic(q, this.cfg.NumOfPartition, this.cfg.NumOfReplica)
	if err != nil && util.ContainStr(err.Error(), "already exists") {
		this.q.Store(q, client)
		return nil
	}
	this.q.Store(q, client)
	return err
}

func (this *KafkaQueue) Push(q string, data []byte) error {

	if data == nil || len(data) == 0 {
		panic(errors.New("invalid data"))
	}

	var client *kgo.Client
	var err error
	k, ok := this.q.Load(q)
	if !ok {
		err = this.Init(q)
		if err != nil {
			panic(err)
		}
		k, ok = this.q.Load(q)
	}

	client, ok = k.(*kgo.Client)
	if !ok || client == nil {
		panic(errors.New("invalid client"))
	}

	messages := []*kgo.Record{}

	msg := &kgo.Record{}
	msg.Topic = q
	msg.Timestamp = time.Now()
	msg.Key = util.UnsafeStringToBytes(util.GetUUID())
	msg.Value = data

	messages = append(messages, msg)

	result := client.ProduceSync(context.Background(), messages...)
	return result.FirstErr()
}

func (this *KafkaQueue) Pop(q string, t time.Duration) (data []byte, timeout bool) {

	q1, ok := this.q.Load(q)
	if !ok || q1 == nil {
		return nil, true
	}

	qConfig, ok := queue.GetConfigByUUID(q)
	if !ok {
		qConfig, ok = queue.SmartGetConfig(q)
	}

	if ok {
		cConfig := queue.NewConsumerConfig(qConfig.ID, "default", "default")
		cConfig.AutoCommitOffset = true
		consumer, err := this.AcquireConsumer(qConfig, cConfig)
		if err != nil {
			panic(err)
		}
		msg, _, err := consumer.FetchMessages(&queue.Context{}, 1)
		if len(msg) > 0 {
			return msg[0].Data, false
		}
	}

	return nil, true
}

func (this *KafkaQueue) Close(string) error {
	return nil
}

func (this *KafkaQueue) Depth(q string) int64 {
	so, err := this.adminClient.ListStartOffsets(context.Background(), q)
	if err != nil {
		panic(err)
	}
	eo, err := this.adminClient.ListEndOffsets(context.Background(), q)
	if err != nil {
		panic(err)
	}
	return eo.Offsets()[q][0].At - so.Offsets()[q][0].At
}

func (this *KafkaQueue) LatestOffset(k *queue.QueueConfig) queue.Offset {

	offset1, err := this.adminClient.ListEndOffsets(context.Background(), k.ID)
	if err != nil {
		log.Error(k.Name, ", error on get offset:", offset1)
		panic(err)
	}
	return queue.NewOffset(0, offset1[k.ID][0].Offset)
}

func (this *KafkaQueue) GetQueues() []string {
	q := []string{}
	hash := hashset.New()
	this.q.Range(func(key, value interface{}) bool {
		k := util.ToString(key)
		hash.Add(k)
		q = append(q, k)
		return true
	})

	log.Debugf("try to load from kafka")
	topics, err := this.adminClient.ListTopics(context.Background())
	if err == nil && topics != nil {
		q1 := topics.TopicsList().Topics()
		log.Tracef("load queues from kafka %v", q1)
		for _, v := range q1 {
			if !hash.Contains(v) {
				q = append(q, v)
			}
		}
	}
	return q
}

var ignoredError = []string{"NOT_COORDINATOR", "UNKNOWN_TOPIC_OR_PARTITION"}

func (this *KafkaQueue) GetOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig) (queue.Offset, error) {

	os, err := this.adminClient.FetchOffsetsForTopics(context.Background(), getGroupForKafka(consumer.Group, k.ID), k.ID)
	if err != nil {
		if util.ContainsAnyInArray(err.Error(), ignoredError) {
			if global.Env().IsDebug {
				log.Debugf("err on get offset %v %v %v", k.ID, err)
			}
			return queue.NewOffset(0, 0), nil
		}
		panic(err)
	}

	var offset int64 = 0
	res, ok := os.Lookup(k.ID, 0)
	if ok {
		offset = res.Offset.At
	}

	if offset < 0 {
		offset = 0
	}

	str := queue.NewOffset(0, offset)

	if global.Env().IsDebug {
		log.Debugf("get offset %v, %v, %v, %v, %v", k.ID, str, err, res, os)
	}
	return str, nil
}

func (this *KafkaQueue) GetStorageSize(k string) uint64 {
	topic := kadm.TopicsSet{}
	topic.Add(k, 0)
	res, err := this.adminClient.DescribeAllLogDirs(context.Background(), topic)
	if err != nil {
		log.Errorf("get storage size for %v, error:%v", k, err)
		return 0
	}
	return uint64(res[0].Size())
}

func (this *KafkaQueue) DeleteOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig) error {

	_, err := this.CommitOffset(k, consumer, queue.NewOffset(0, 0))
	topic := kadm.TopicsSet{}
	topic.Add(k.ID)
	_, err = this.adminClient.DeleteOffsets(context.Background(), getGroupForKafka(consumer.Group, k.ID), topic)
	return err
}

func (this *KafkaQueue) CommitOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig, offset queue.Offset) (bool, error) {

	group := getGroupForKafka(consumer.Group, k.ID)

	cor, ok := this.consumers.Load(group)
	if ok {
		ins, ok := cor.(*Consumer)
		if ok {
			err := ins.CommitOffset(offset)
			if err != nil {
				panic(err)
			}
			return true, nil
		}
	}

	res1, err := this.adminClient.FetchOffsets(context.Background(), "default")
	if err != nil {
		panic(err)
	}
	leaderO := res1["default"][0].Offset.LeaderEpoch

	offsets := kadm.Offsets{}
	offsets.AddOffset(k.ID, int32(offset.Segment), offset.Position, leaderO)
	res, err := this.adminClient.CommitOffsets(context.Background(), group, offsets)
	if err != nil {
		log.Error(util.MustToJSON(res))
		panic(err)
	}
	return true, nil
}

func (this *KafkaQueue) AcquireProducer(cfg *queue.QueueConfig) (queue.ProducerAPI, error) {

	v, ok := this.producers.Load(cfg.ID)
	if ok {
		f, ok := v.(queue.ProducerAPI)
		if ok {
			return f, nil
		}
	}

	_, ok = queue.GetConfigByUUID(cfg.ID)
	if !ok {
		_, err := queue.RegisterConfig(cfg)
		if err != nil {
			panic(err)
		}
	}

	producerID := util.GetUUID()
	opts := []kgo.Opt{
		kgo.AllowAutoTopicCreation(),
		kgo.ProducerBatchMaxBytes(this.cfg.ProducerBatchMaxBytes),
		kgo.MaxBufferedRecords(this.cfg.MaxBufferedRecords),
		kgo.InstanceID(producerID),
		kgo.SessionTimeout(30 * time.Second),
		kgo.HeartbeatInterval(10 * time.Second),
		kgo.RetryTimeout(10 * time.Second),
		kgo.RebalanceTimeout(10 * time.Second),
		kgo.TransactionTimeout(10 * time.Second),
		kgo.RequestTimeoutOverhead(10 * time.Second),
		kgo.ClientID(global.Env().SystemConfig.NodeConfig.ID),
	}

	if this.cfg.ManualFlushing {
		opts = append(opts, kgo.ManualFlushing())
	}

	producer := &Producer{ID: producerID, client: this.newClient(opts), cfg: cfg}
	this.producers.Store(cfg.ID, producer)
	if global.Env().IsDebug {
		log.Infof("acquired producer:%v, %v, %v", cfg.Name, cfg.ID, producer.ID)
	}
	return producer, nil
}

func (this *KafkaQueue) ReleaseProducer(k *queue.QueueConfig, producer queue.ProducerAPI) error {
	//if this.producer != nil {
	//	this.producer.Close()
	//	this.producer = nil
	//}
	return nil
}
