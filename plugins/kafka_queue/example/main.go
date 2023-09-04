/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package main

import (
	"context"
	"fmt"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	//"github.com/twmb/franz-go/pkg/kmsg"
	"time"
)

func onPartitionRevoked(_ context.Context, _ *kgo.Client, _ map[string][]int32) {
	fmt.Println("hello onPartitionRevoked")
	time.Sleep(30 * time.Second)
	//k.cleanupFn() //This func waits all flying batches be written to database and committed to Kafka. UNKNOWN_MEMBER_ID is throwed during this func. My question is, has this client leaved the group at this point?
}

func main() {
	seeds := []string{"192.168.3.185:9092"}
	// One client can both produce and consume!
	// Consuming can either be direct (no consumer group), or through a group. Below, we use a group.
	topic := "aa59d67c2123f094d0d6798ffe651c4d"
	group := "group-001_cj69e6o05f5jhm77ivvg"

	offset1 := kgo.NewOffset()
	offset1.At(1)

	cl, err := kgo.NewClient(
		kgo.SessionTimeout(10*time.Second),
		kgo.HeartbeatInterval(10*time.Second),
		kgo.RetryTimeout(10*time.Second),
		kgo.RebalanceTimeout(10*time.Second),
		kgo.TransactionTimeout(10*time.Second),
		kgo.RequestTimeoutOverhead(10*time.Second),
		kgo.BlockRebalanceOnPoll(),
		kgo.ClientID("my_client"),
		kgo.ConsumeResetOffset(offset1),
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(group),
		//kgo.ConsumePartitions()
		//kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		//kgo.FetchMinBytes(1),
		//kgo.OnPartitionsRevoked(onPartitionRevoked),

	)
	if err != nil {
		panic(err)
	}

	defer cl.Close()

	ctx := context.Background()
	//
	//// 1.) Producing a message
	//// All record production goes through Produce, and the callback can be used
	//// to allow for synchronous or asynchronous production.
	//var wg sync.WaitGroup
	//wg.Add(1)
	//record := &kgo.Record{Topic: "foo", Value: []byte("bar")}
	//cl.Produce(ctx, record, func(_ *kgo.Record, err error) {
	//	defer wg.Done()
	//	if err != nil {
	//		fmt.Printf("record had a produce error: %v\n", err)
	//	}
	//
	//})
	//wg.Wait()
	//
	//// Alternatively, ProduceSync exists to synchronously produce a batch of records.
	//if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
	//	fmt.Printf("record had a produce error while synchronously producing: %v\n", err)
	//}

	var adm *kadm.Client = kadm.NewClient(cl)

	x, _ := adm.ListGroups(context.Background(), topic)
	fmt.Println(x)

	// With the admin client, we can either FetchOffsets to fetch strictly
	// the prior committed offsets, or FetchOffsetsForTopics to fetch
	// offsets for topics and have -1 offset defaults for topics that are
	// not yet committed. We use FetchOffsetsForTopics here.
	seeds1 := kgo.SeedBrokers(seeds...)
	cl, err = kgo.NewClient(seeds1, kgo.ConsumePartitions(
		map[string]map[int32]kgo.Offset{
			topic: {0: kgo.NewOffset().At(1024)},
		}))
	if err != nil {
		panic(err)
	}

	//groups,err:=adm.DescribeGroups(context.Background())
	////fmt.Println(util.MustToJSON(groups),err)
	//for k,v:=range groups{
	//	//fmt.Println(k,v)
	//	par:=v.AssignedPartitions()
	//	fmt.Println(k,par)
	//}

	//
	//// With the admin client, we can either FetchOffsets to fetch strictly
	//// the prior committed offsets, or FetchOffsetsForTopics to fetch
	//// offsets for topics and have -1 offset defaults for topics that are
	//// not yet committed. We use FetchOffsetsForTopics here.
	//os, err := adm.FetchOffsetsForTopics(context.Background(), group, topic)
	//if err != nil {
	//	panic(errors.Errorf("unable to fetch group offsets: %v", err))
	//}
	//
	//fmt.Println("fetching offset:", os.KOffsets())
	//
	////update offset
	//newOffset := kadm.Offsets{}
	//newOffset.AddOffset(topic, int32(0), 113, 0)
	//fmt.Println("updating offset:", newOffset.KOffsets())
	//err = adm.CommitAllOffsets(context.Background(), group, newOffset)
	//fmt.Println("commit offset:",  err)
	//os, err = adm.FetchOffsetsForTopics(context.Background(), group, topic)
	//if err != nil {
	//	panic(errors.Errorf("unable to fetch group offsets: %v", err))
	//}
	//fmt.Println("fetching new offset:", os.KOffsets())
	//
	//r := kadm.OffsetForLeaderEpochRequest{}
	//r.Add(topic, 0, 0)
	//res1, err := adm.OffetForLeaderEpoch(context.Background(), r)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println("Topic: %v,last:%v",topic, res1[topic][0].EndOffset)
	////
	////newOffset = kadm.Offsets{}
	////newOffset.AddOffset(topic, int32(0), 144, res1[topic][0].LeaderEpoch)
	////fmt.Println("updating offset:", newOffset.KOffsets())
	////err = adm.CommitAllOffsets(context.Background(), group, newOffset)
	////fmt.Println("commit offset:",  err)
	//os2, err := adm.FetchOffsetsForTopics(context.Background(), group, topic)
	//if err != nil {
	//	panic(errors.Errorf("unable to fetch group offsets: %v", err))
	//}
	//fmt.Println("fetching new offset:", os2.KOffsets())
	//
	//os2.Exit(1)
	//
	//cl1, err := kgo.NewClient(kgo.SeedBrokers(seeds...), kgo.ConsumePartitions(os.KOffsets()))
	//if err != nil {
	//	panic(errors.Errorf("unable to create client: %v", err))
	//}
	//defer cl.Close()
	//
	//fmt.Println("Waiting for one record...")
	//fs := cl1.PollRecords(context.Background(), 1)
	//
	//// kadm has two offset committing functions: CommitOffsets and
	//// CommitAllOffsets. The former allows you to check per-offset error
	//// codes, the latter returns the first error of any in the offsets.
	////
	//// We use the latter here because we are only committing one offset,
	//// and even if we committed more, we do not care about partial failed
	//// commits. Use the former if you care to check per-offset commit
	//// errors.
	//if err := adm.CommitAllOffsets(context.Background(), group, kadm.OffsetsFromFetches(fs)); err != nil {
	//	panic(errors.Errorf("unable to commit offsets: %v", err))
	//}

	//r := fs.Records()[0]
	//fmt.Printf("Successfully committed record on partition %d at offset %d!\n", r.Partition, r.Offset)

	//////update offset, not working
	//offset :=map[string]map[int32]kgo.EpochOffset{}
	//offset[topic] = map[int32]kgo.EpochOffset{}
	//offset[topic][0]=kgo.EpochOffset{Offset: 1024, Epoch: 0}
	////cl.CommitOffsetsSync(context.Background(),offset, func(client *kgo.Client, request *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
	////	fmt.Println("commit offset:",err)
	////})
	//
	//cl.SetOffsets(offset) //not working

	//cl.SetOffsets(offset)

	// 2.) Consuming messages from a topic
	for {
		fetches := cl.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			// All errors are retried internally when fetching, but non-retriable errors are
			// returned from polls so that users can notice and take action.
			panic(fmt.Sprint(errs))
		}

		// We can iterate through a record iterator...
		iter := fetches.RecordIter()
		count := 0
		for !iter.Done() {
			record := iter.Next()
			fmt.Println(string(record.Key), "from an iterator!,offset:", record.Offset, ",partition:", record.Partition)
			time.Sleep(1 * time.Second)
			count++
			////offset.
			//if count>5{
			//	break
			//}
		}

		//// or a callback function.
		//fetches.EachPartition(func(p kgo.FetchTopicPartition) {
		//	for _, record := range p.Records {
		//		fmt.Println(string(record.Key), "from range inside a callback!")
		//	}
		//
		//	// We can even use a second callback!
		//	p.EachRecord(func(record *kgo.Record) {
		//		fmt.Println(string(record.Key), "from a second callback!")
		//	})
		//})
	}

}
