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

/* Copyright © INFINI Ltd. All rights reserved.
* web: https://infinilabs.com
* mail: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"crypto/tls"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/module"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"math"
	"net"
	"sync"
	"time"
)

func init() {
	module.RegisterSystemModule(&KafkaQueue{})
}

// Config defines Kafka connectivity, topic creation, and buffering limits.
// The delivery-order budget applies separately to each acquired consumer session.
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
	// OrderMemoryBudgetBytes limits interval blocks, excluding SDK buffers and maps.
	// Zero selects the 64 MiB default. Exhaustion expires the read session without committing.
	OrderMemoryBudgetBytes uint64 `config:"order_memory_budget_bytes"`
}

// const PLAIN_MECHANISM = "PLAIN"//
const SCRAM_SHA_256_Mechanism = "SCRAM-SHA-256"
const SCRAM_SHA_512_Mechanism = "SCRAM-SHA-512"

// KafkaQueue implements queue operations with topic clients and independent consumer sessions.
//
// Business confirmations must reach their original instance. Group-name routing alone
// can send a delayed confirmation to a replacement with different delivery history.
//
// mu serializes client registration, local ownership changes, administrative resets,
// and shutdown admission. Stop releases it before closing individual consumers.
type KafkaQueue struct {
	cfg         *Config
	q           sync.Map // Topic name -> client used by Push.
	consumers   sync.Map // Kafka group name -> business instance holding the local slot.
	producers   sync.Map // Topic name -> reusable producer.
	adminClient *kadm.Client
	sessions    sync.Map // Negative Version -> original business instance, until lease cleanup succeeds.
	instances   sync.Map // All acquired instances, including prepared, browsing, and cleanup retries.
	mu          sync.Mutex
	stopping    bool // Once set under mu, no factory or business claim can reopen the backend.
}

func (this *KafkaQueue) newClient(opt []kgo.Opt) *kgo.Client {
	opts := []kgo.Opt{
		kgo.SeedBrokers(this.cfg.Brokers...),
	}

	if !this.cfg.Compression {
		opts = append(opts, kgo.ProducerBatchCompression(kgo.NoCompression()))
	}

	if this.cfg.Username != "" {
		if this.cfg.TLS {
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

// Helper function to derive the shared group name for reads, ownership, and management.
// Each framework group tracks recovery positions independently for each topic.
func getGroupForKafka(group, topic string) string {
	return group + "_" + topic
}

// ReleaseConsumer closes only the supplied instance after checking its backend, queue, and group.
//
// Releasing by group name can remove a replacement's slot or lease. Close keeps
// ownership tied to the original instance and can retry failed resource cleanup.
func (backend *KafkaQueue) ReleaseConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig, instance queue.ConsumerAPI) error {
	if instance == nil {
		return nil
	}
	c, ok := instance.(*Consumer)
	if !ok || c.owner != backend || qconfig == nil || consumer == nil || c.qCfg.ID != qconfig.ID || c.cCfg.Group != consumer.Group {
		return fmt.Errorf("kafka consumer release does not match its owner")
	}
	return c.Close()
}

// AcquireConsumer creates an independent session and snapshots its configuration without joining Kafka.
//
// The first read opens the client. This leaves ResetOffset free to select browsing
// and avoids mutating configuration shared with another caller.
//
// Every instance receives a process-local negative Version and registers under mu.
// Stop can therefore close it even before its first read. Business ownership is
// reserved later, when the first business read starts.
func (backend *KafkaQueue) AcquireConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig) (queue.ConsumerAPI, error) {
	if qconfig == nil || qconfig.ID == "" || consumer == nil {
		return nil, fmt.Errorf("invalid Kafka consumer configuration")
	}
	if consumer.AutoResetOffset != "" && consumer.AutoResetOffset != "earliest" && consumer.AutoResetOffset != "latest" {
		return nil, fmt.Errorf("invalid Kafka auto_reset_offset %q", consumer.AutoResetOffset)
	}
	sequence := sessionSequence.Add(1)
	if sequence <= 0 {
		return nil, fmt.Errorf("kafka session identifiers exhausted")
	}
	ioCtx, cancel := context.WithCancel(context.Background())
	budget := uint64(defaultOrderMemoryBudget)
	if backend.cfg != nil && backend.cfg.OrderMemoryBudgetBytes > 0 {
		budget = backend.cfg.OrderMemoryBudgetBytes
	}
	c := &Consumer{owner: backend, qCfg: &queue.QueueConfig{ID: qconfig.ID, Name: qconfig.Name}, cCfg: consumer.Clone(), version: -sequence, ioCtx: ioCtx, cancel: cancel, order: deliveryOrder{budget: budget}}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.stopping {
		cancel()
		return nil, fmt.Errorf("kafka queue is stopping")
	}
	backend.instances.Store(c, struct{}{})
	return c, nil
}

func (this *KafkaQueue) Destroy(k string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()

	_, err := this.adminClient.DeleteTopics(ctx, k)
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
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	if !this.cfg.Enabled {
		return
	}

	opts := []kgo.Opt{
		kgo.InstanceID(global.Env().SystemConfig.NodeConfig.ID),
	}

	client := this.newClient(opts)
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

// Stop rejects new clients and claims, then closes all registered instances and shared clients.
//
// Browsing and prepared instances also own lifecycle state, so the business-slot
// registry alone cannot identify everything that needs closing.
//
// The instance snapshot is taken under mu. Cleanup runs after releasing mu because
// Consumer.Close acquires it to release ownership. Failed releases remain registered
// so a subsequent Close or Stop can retry them.
func (backend *KafkaQueue) Stop() error {
	backend.mu.Lock()
	backend.stopping = true
	instances := []*Consumer{}
	backend.instances.Range(func(key, _ interface{}) bool { instances = append(instances, key.(*Consumer)); return true })
	backend.mu.Unlock()
	var result error
	for _, c := range instances {
		if err := c.Close(); err != nil {
			result = err
		}
	}
	backend.producers.Range(func(_, value interface{}) bool {
		value.(*Producer).client.Close()
		return true
	})
	backend.q.Range(func(_, value interface{}) bool {
		value.(*kgo.Client).Close()
		return true
	})
	if backend.adminClient != nil {
		backend.adminClient.Close()
	}
	return result
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()

	res, err := this.adminClient.CreateTopic(ctx, int32(partition), int16(replica), nil, topic)
	log.Tracef("create topic:%v, %v, %v, %v,%v", topic, partition, replica, res, err)
	return err
}

// Init creates the topic when needed and retains a client for Push.
//
// Holding mu through creation and registration prevents a concurrent Stop from
// taking its client snapshot before this client becomes visible.
func (backend *KafkaQueue) Init(q string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.stopping {
		return fmt.Errorf("kafka queue is stopping")
	}
	if c, ok := backend.q.Load(q); ok && c != nil {
		client, ok := c.(*kgo.Client)
		if ok && client != nil {
			return nil
		}
	}
	opts := []kgo.Opt{
		kgo.ConsumeTopics(q),
	}
	client := backend.newClient(opts)
	err := backend.createTopic(q, backend.cfg.NumOfPartition, backend.cfg.NumOfReplica)
	if err != nil && util.ContainStr(err.Error(), "already exists") {
		backend.q.Store(q, client)
		return nil
	}
	backend.q.Store(q, client)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()

	result := client.ProduceSync(ctx, messages...)
	return result.FirstErr()
}

// Pop returns one message from the default group only after its explicit confirmation succeeds.
//
// The temporary consumer needs an explicit commit and release so a later Pop
// resumes from the saved position without retaining an ownership slot.
//
// A positive wait bounds polling, rounded up to milliseconds. Nonpositive values
// keep polling, including the public queue.Pop sentinel -1. After acquisition, every
// exit attempts cleanup. Operation or cleanup errors panic because this signature
// has no error result.
func (backend *KafkaQueue) Pop(topic string, wait time.Duration) ([]byte, bool) {
	cfg, ok := queue.GetConfigByUUID(topic)
	if !ok {
		cfg, ok = queue.SmartGetConfig(topic)
	}
	if !ok {
		return nil, true
	}
	consumerCfg := queue.NewConsumerConfig(cfg.ID, "default", "default")
	consumerCfg.FetchMaxWaitMs = 1000
	if wait > 0 {
		consumerCfg.FetchMaxWaitMs = int64((wait-1)/time.Millisecond + 1)
	}
	consumerCfg.FetchMaxMessages = 1
	consumerCfg.AutoCommitOffset = false
	instance, err := backend.AcquireConsumer(cfg, consumerCfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := backend.ReleaseConsumer(cfg, consumerCfg, instance); err != nil {
			panic(err)
		}
	}()
	for {
		messages, timeout, err := instance.FetchMessages(&queue.Context{}, 1)
		if err != nil {
			panic(err)
		}
		if len(messages) == 0 {
			if wait > 0 {
				return nil, timeout
			}
			continue
		}
		if err := instance.CommitOffset(messages[0].NextOffset); err != nil {
			panic(err)
		}
		return messages[0].Data, false
	}
}

func (this *KafkaQueue) Close(string) error {
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()

	topics, err := this.adminClient.ListTopics(ctx)
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

func (this *KafkaQueue) GetStorageSize(k string) uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()

	topic := kadm.TopicsSet{}
	topic.Add(k, 0)
	res, err := this.adminClient.DescribeAllLogDirs(ctx, topic)
	if err != nil {
		log.Errorf("get storage size for %v, error:%v", k, err)
		return 0
	}
	return uint64(res[0].Size())
}

// DeleteOffset removes this group's saved positions for all actual partitions of the topic.
//
// This is an administrative operation. The caller must stop the group first.
// A local business slot rejects it immediately, and Kafka enforces its group rules
// for other members. Deletion does not first commit an artificial partition-zero position.
//
// mu excludes local claims until the request finishes. Failure can include a partial
// or unknown broker outcome, so an error does not prove all positions are unchanged.
func (backend *KafkaQueue) DeleteOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig) error {
	if k == nil || consumer == nil {
		return fmt.Errorf("invalid Kafka consumer configuration")
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.stopping {
		return fmt.Errorf("kafka queue is stopping")
	}
	group := getGroupForKafka(consumer.Group, k.ID)
	if _, ok := backend.consumers.Load(group); ok {
		return fmt.Errorf("stop the Kafka consumer group before deleting offsets")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	partitions, err := backend.adminClient.ListTopics(ctx, k.ID)
	if err != nil {
		return err
	}
	topic, ok := partitions[k.ID]
	if !ok || topic.Err != nil {
		return fmt.Errorf("kafka topic metadata unavailable")
	}
	targets := kadm.TopicsSet{}
	for part, detail := range topic.Partitions {
		if detail.Err != nil {
			return detail.Err
		}
		targets.Add(k.ID, part)
	}
	response, err := backend.adminClient.DeleteOffsets(ctx, group, targets)
	if err != nil {
		return err
	}
	if err = response.Error(); err != nil {
		return err
	}
	if len(response[k.ID]) != len(topic.Partitions) {
		return fmt.Errorf("kafka delete response omitted a partition")
	}
	return nil
}

// CommitOffset routes a business acknowledgement to its original session or resets one administrative position.
//
// A business confirmation represents a delivered prefix, while an administrative
// reset changes one physical partition. The Version sign keeps these operations separate.
//
// Negative versions must match a live route for this queue and group. Nonnegative
// versions require a stopped group and an in-range partition address. mu excludes
// local claims during management. A timeout can leave the broker outcome unknown.
func (backend *KafkaQueue) CommitOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig, offset queue.Offset) (bool, error) {
	if k == nil || consumer == nil || offset.Segment < 0 || offset.Segment > math.MaxInt32 || offset.Position < 0 {
		return false, fmt.Errorf("invalid Kafka commit address")
	}
	if offset.Version < 0 {
		value, ok := backend.sessions.Load(offset.Version)
		if !ok {
			return false, fmt.Errorf("unknown or closed Kafka acknowledgement session")
		}
		c := value.(*Consumer)
		if c.qCfg.ID != k.ID || c.cCfg.Group != consumer.Group {
			return false, fmt.Errorf("kafka acknowledgement belongs to another queue or group")
		}
		err := c.CommitOffset(offset)
		return err == nil, err
	}
	// Keep local claims excluded until the administrative broker request completes.
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.stopping {
		return false, fmt.Errorf("kafka queue is stopping")
	}
	group := getGroupForKafka(consumer.Group, k.ID)
	if _, ok := backend.consumers.Load(group); ok {
		return false, fmt.Errorf("stop the Kafka consumer group before resetting offsets")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	starts, ends, err := backend.positionBounds(ctx, k.ID)
	if err != nil {
		return false, err
	}
	part := int32(offset.Segment)
	start, ok := starts[part]
	end := ends[part]
	if !ok || offset.Position < start || offset.Position > end {
		return false, fmt.Errorf("kafka reset position is out of range")
	}
	offsets := kadm.Offsets{}
	offsets.AddOffset(k.ID, part, offset.Position, -1)
	response, err := backend.adminClient.CommitOffsets(ctx, group, offsets)
	if err != nil {
		return false, fmt.Errorf("kafka reset failed; its outcome can be unknown: %w", err)
	}
	if err = response.Error(); err != nil {
		return false, err
	}
	return true, nil
}

// AcquireProducer returns the reusable producer for a topic or registers a new one.
//
// Creation and registration hold mu so Stop cannot miss a newly created client.
// After shutdown begins, acquisition fails even when a cached producer exists.
func (backend *KafkaQueue) AcquireProducer(cfg *queue.QueueConfig) (queue.ProducerAPI, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.stopping {
		return nil, fmt.Errorf("kafka queue is stopping")
	}
	if cfg == nil || cfg.ID == "" {
		return nil, fmt.Errorf("invalid Kafka producer configuration")
	}

	v, ok := backend.producers.Load(cfg.ID)
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
		kgo.ProducerBatchMaxBytes(backend.cfg.ProducerBatchMaxBytes),
		kgo.MaxBufferedRecords(backend.cfg.MaxBufferedRecords),
		kgo.InstanceID(producerID),
		kgo.SessionTimeout(30 * time.Second),
		kgo.HeartbeatInterval(10 * time.Second),
		kgo.RetryTimeout(10 * time.Second),
		kgo.RebalanceTimeout(10 * time.Second),
		kgo.TransactionTimeout(10 * time.Second),
		kgo.RequestTimeoutOverhead(10 * time.Second),
		kgo.ClientID(global.Env().SystemConfig.NodeConfig.ID),
	}

	if backend.cfg.ManualFlushing {
		opts = append(opts, kgo.ManualFlushing())
	}

	producer := &Producer{ID: producerID, client: backend.newClient(opts), cfg: cfg}
	backend.producers.Store(cfg.ID, producer)
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
