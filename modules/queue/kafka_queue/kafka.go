/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/segmentio/kafka-go"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	memQueue "infini.sh/framework/lib/lock_free/queue"
	"infini.sh/framework/modules/queue/common"
	"github.com/segmentio/kafka-go/sasl/scram"
	"sync"
	"time"
)

type KafkaQueue struct {
	Enabled bool `config:"enabled"`
	Default bool `config:"default"`

	AutoCreateTopic bool `config:"auto_create_topic"`
	NumOfPartition  int  `config:"num_of_partition"`
	NumOfReplica    int  `config:"num_of_replica"`

	BatchSize        int      `config:"batch_size"`
	BatchTimeoutInMs int      `config:"batch_timeout_in_ms"`
	RequiredAcks     int      `config:"required_acks"`
	Brokers          []string `config:"brokers"`
	Username         string   `config:"username"`
	Password         string   `config:"password"`

	//share
	msgPool sync.Pool

	taskContext context.Context

	q sync.Map

	locker sync.RWMutex

	wPool sync.Pool

	rPool sync.Pool

	sharedTransport *kafka.Transport
	dialer          *kafka.Dialer
}

type KafkaTopic struct {

}

func (this *KafkaQueue) Setup(config *config.Config) {

	this.q = sync.Map{}
	this.Enabled = true
	this.NumOfPartition = 1
	this.NumOfReplica = 1
	this.AutoCreateTopic = true

	ok, err := env.ParseConfig("kafka_queue", &this)
	if ok && err != nil {
		panic(err)
	}

	if !this.Enabled {
		return
	}

	common.InitQueueMetadata()

	this.taskContext = context.Background()

	this.msgPool = sync.Pool{
		New: func() interface{} {
			return &kafka.Message{}
		},
	}

	// Transports are responsible for managing connection pools and other resources,
	// it's generally best to create a few of these and share them across your
	// application.
	this.sharedTransport = &kafka.Transport{
		//SASL: mechanism,
	}

	if this.Username!=""{
		mechanism, err := scram.Mechanism(scram.SHA512, this.Username, this.Password)
		if err != nil {
			panic(err)
		}

		this.sharedTransport.SASL=mechanism

		//client := &kafka.Client{
		//	Addr:      kafka.TCP(this.Brokers...),
		//	Timeout:   10 * time.Second,
		//	Transport: sharedTransport,
		//}

		this.dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: mechanism,
		}
	}

	//req := kafka.FetchRequest{}
	//res:=client.Fetch(context.Background(),&req)
	//w := kafka.NewWriter(kafka.WriterConfig{
	//	Brokers:      this.Brokers,
	//	Topic:        q,
	//	BatchSize:    this.BatchSize,
	//	BatchTimeout: time.Duration(this.BatchTimeoutInMs) * time.Millisecond,
	//	MaxAttempts:  10,
	//	RequiredAcks: this.RequiredAcks,
	//})
	//
	//w.AllowAutoTopicCreation = true
	//w.Balancer = &kafka.Hash{}

	//if this.AutoCreateTopic{
	//	err:=this.createTopic(q,this.NumOfPartition,this.NumOfReplica)
	//	if err!=nil{
	//		log.Error(err)
	//	}
	//}


	//r := kafka.NewReader(kafka.ReaderConfig{
	//	Brokers:        []string{"localhost:9092","localhost:9093", "localhost:9094"},
	//	GroupID:        "consumer-group-id",
	//	Topic:          "topic-A",
	//	Dialer:         this.dialer,
	//})


	this.wPool= sync.Pool{
		New: func() interface{} {
			w:=kafka.Writer{
				Addr:     kafka.TCP(this.Brokers...),
				Transport: this.sharedTransport,
				Balancer: &kafka.LeastBytes{},
				AllowAutoTopicCreation: true,
				BatchSize:    this.BatchSize,
				BatchTimeout: time.Duration(this.BatchTimeoutInMs) * time.Millisecond,
				MaxAttempts:  10,
				RequiredAcks: kafka.RequireOne,
			}
			return &w
		},
	}

	this.rPool= sync.Pool{
		New: func() interface{} {
			r:=kafka.NewReader(kafka.ReaderConfig{
				Brokers:   this.Brokers,
			})
			return &r
		},
	}

	queue.Register("kafka", this)

	if this.Default {
		queue.RegisterDefaultHandler(this)
	}

}

func (this *KafkaQueue) Start() error {
	return nil
}

func (this *KafkaQueue) Stop() error {
	common.PersistQueueMetadata()
	return nil
}

func (this *KafkaQueue) Name() string {
	return "kafka_queue"
}

//func (this *KafkaQueue) createTopic(topic string,partition,replica int)error {
//
//	// to create topics when auto.create.topics.enable='false'
//	conn, err := kafka.Dial("tcp", this.Brokers[0])
//	if err != nil {
//		return err
//	}
//	defer conn.Close()
//
//	controller, err := conn.Controller()
//	if err != nil {
//		return err
//	}
//	var controllerConn *kafka.Conn
//	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
//	if err != nil {
//		return err
//	}
//	defer controllerConn.Close()
//
//	topicConfigs := []kafka.TopicConfig{
//		{
//			Topic:             topic,
//			NumPartitions:     partition,
//			ReplicationFactor: replica,
//		},
//	}
//
//	err = controllerConn.CreateTopics(topicConfigs...)
//	if err != nil {
//		return err
//	}
//
//	return err
//}

func (this *KafkaQueue) Init(q string) error {



	//this.q.Store(q, w)

	return nil
}



func (this *KafkaQueue) Push(q string, data []byte) error {
	//q1, ok := this.q.Load(q)
	//if !ok {
	//	err := this.Init(q)
	//	if err != nil {
	//		panic(err)
	//	}
	//	q1, _ = this.q.Load(q)
	//}

	mq:=this.wPool.Get().(*kafka.Writer)
	defer this.wPool.Put(mq)

	//retryTimes:=0
	da := []byte(string(data)) //TODO memory copy

	messages := []kafka.Message{}

	msg := this.msgPool.Get().(*kafka.Message)
	msg.Topic=q
	defer this.msgPool.Put(msg)
	msg.Key = util.UnsafeStringToBytes(util.IntToString(int(util.GetIncrementID(q))))
	msg.Value = da

	messages = append(messages, *msg)

	//RETRY:
	//ok,_=mq.Put(da)
	//log.Error("write topic:",mq.Topic)

	//this.locker.Lock()
	//err := mq.WriteMessages(this.taskContext, messages...)
	//this.locker.Unlock()

	var err error
	const retries = 3
	start:=time.Now()
	for i := 0; i < retries; i++ {
		//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		//defer cancel()

		// attempt to create topic prior to publishing the message
		//this.locker.Lock()
		err = mq.WriteMessages(context.Background(), messages...)
		//this.locker.Unlock()

		if this.BatchTimeoutInMs>0&&time.Since(start).Milliseconds()>int64(this.BatchTimeoutInMs){
			return errors.New("timeout on push")
		}

		if  errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, kafka.GroupCoordinatorNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(time.Millisecond * 250)
			log.Errorf("error:%v, retry",err)
			continue
		}

		if err != nil {
			log.Errorf("unexpected error %v", err)
			return err
		}
		return nil
	}

	//if err := mq.Close(); err != nil {
	//	log.Errorf("failed to close writer:", err)
	//	return err
	//}

	//if !ok{
	//	if retryTimes>10{
	//		stats.Increment("mem_queue","dead_retry")
	//		return capacityFull
	//	}else{
	//		retryTimes++
	//		runtime.Gosched()
	//		log.Debugf("memory_queue %v of %v, sleep 1s",mq.Quantity(),mq.Capaciity())
	//		time.Sleep(1000*time.Millisecond)
	//		stats.Increment("mem_queue","retry")
	//		goto RETRY
	//	}
	//}
	return err
}

//var capacityFull =errors.New("queue capacity full")

func (this *KafkaQueue) Pop(q string, t time.Duration) (data []byte, timeout bool) {
	//queue,ok:=this.q.Load(q)
	//if !ok||queue==nil{
	//	return nil, true
	//}
	//
	//mq,ok:=queue.(*kafka.Writer{})
	//if !ok{
	//	panic("invalid memory queue")
	//}
	//
	//v,ok,_:=mq.Get()
	//if ok&&v!=nil{
	//	d,ok:=v.([]byte)
	//	if ok{
	//		return d,false
	//	}
	//}

	qConfig, ok := queue.GetConfigByUUID(q)
	if !ok {
		qConfig, ok = queue.GetConfigByKey(q)
	}

	if ok {
		cConfig := queue.NewConsumerConfig("default", "default")
		cConfig.FetchMaxMessages = 123
		_, msg, _, _ := this.Consume(qConfig, cConfig, "")

		log.Error("received:", len(msg))

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

	log.Error("get depth:")

	q1, ok := this.q.Load(q)
	if ok {
		mq, ok := q1.(*memQueue.EsQueue)
		if !ok {
			panic("invalid memory queue")
		}
		return int64(mq.Quantity())
	}
	return 0
}

func (this *KafkaQueue) Consume(q *queue.QueueConfig, consumer *queue.ConsumerConfig, offset string) (*queue.Context, []queue.Message, bool, error) {

	//l := log1.New(os.Stdout, "kafka reader: ", 0)
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: this.Brokers,
		StartOffset: 0,
		Topic:   q.Id,
		GroupID: consumer.Group,
		Dialer:         this.dialer,
		//Logger:  l,
	})

	defer r.Close()

	////set offset
	//r.SetOffset()

	ctx1, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx := &queue.Context{}
	msgs := []queue.Message{}
	count := 0
	byteSize := 0
	start:=time.Now()
	for {
		// the `ReadMessage` method blocks until we receive the next event
		msg, err := r.ReadMessage(ctx1)
		if err != nil {
			//panic("could not read message " + err.Error())
			break
		}
		// after receiving the message, log its value
		//log.Errorf("msgs: %v, received: %v",len(msgs), string(msg.Value))

		offsetStr:=fmt.Sprintf("%v,%v",msg.Partition,msg.Offset)

		m := queue.Message{Offset:offsetStr,Data: msg.Value}
		msgs = append(msgs, m)
		count++
		byteSize += len(msg.Value)
		if (consumer.FetchMaxMessages > 0 && count >= consumer.FetchMaxMessages) ||
			(consumer.FetchMaxBytes > 0 && byteSize >= consumer.FetchMaxBytes) ||(consumer.FetchMaxWaitMs>0&&time.Since(start).Milliseconds()>consumer.FetchMaxWaitMs){
			log.Error(q.Id," hit enough message, ",count, ",partition:",msg.Partition,",offset:",msg.Offset,",byte:",byteSize,",elapsed:",time.Since(start).Milliseconds(),",",util.MustToJSON(consumer))
			break
		}
	}

	//log.Error("start read message")
	//kMsg, err := r.ReadMessage(context.Background())
	//log.Error("end read message")
	//if err != nil {
	//	panic("could not read message " + err.Error())
	//}

	return ctx, msgs, false, nil
}

func (this *KafkaQueue) LatestOffset(string) string {
	log.Error("get offset:")

	return ""
}

func (this *KafkaQueue) GetQueues() []string {
	q := []string{}
	this.q.Range(func(key, value interface{}) bool {
		q = append(q, util.ToString(key))
		return true
	})
	return q
}
