/*
[nsq]: https://github.com/nsqio/nsq
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
 [nsq]: https://github.com/nsqio/nsq
*/

package queue

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"infini.sh/framework/core/stats"
	"io"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
)

// providing a filesystem backed FIFO queue
type DiskBasedQueue struct {
	sync.RWMutex
	metaLock sync.RWMutex

	// 64bit atomic vars need to be first for proper alignment on 32bit platforms
	writePos        int64
	writeSegmentNum int64
	writeFile       *os.File
	writeBuf        bytes.Buffer

	// instantiation time metadata
	name     string
	dataPath string

	maxBytesPerFileRead int64

	exitFlag int32
	needSync bool

	consumerMode bool

	// read related
	depth              int64 //TODO,separate write and read
	readPos            int64
	readSegmentFileNum int64
	nextReadPos        int64
	nextReadFileNum    int64
	reader             *bufio.Reader
	readFile           *os.File
	readChan           chan []byte

	// internal channels
	depthChan         chan int64
	writeChan         chan []byte
	writeResponseChan chan WriteResponse
	emptyChan         chan int
	emptyResponseChan chan error
	exitChan          chan int
	exitSyncChan      chan int

	consumersInReading sync.Map

	cfg          *DiskQueueConfig
}

// NewDiskQueue instantiates a new instance of DiskBasedQueue, retrieving metadata
// from the filesystem and starting the read ahead goroutine
func NewDiskQueueByConfig(name, dataPath string, cfg *DiskQueueConfig) *DiskBasedQueue {
	d := DiskBasedQueue{
		name:     name,
		dataPath: dataPath,
		cfg:                cfg,
		readChan:           make(chan []byte, cfg.ReadChanBuffer),
		depthChan:          make(chan int64),
		writeChan:          make(chan []byte, cfg.WriteChanBuffer),
		writeResponseChan:  make(chan WriteResponse),
		emptyChan:          make(chan int),
		emptyResponseChan:  make(chan error),
		exitChan:           make(chan int),
		exitSyncChan:       make(chan int, 10),
		consumersInReading: sync.Map{},
		metaLock: sync.RWMutex{},
	}

	// no need to lock here, nothing else could possibly be touching this instance
	err := d.retrieveMetaData()
	if err != nil && !os.IsNotExist(err) {
		log.Errorf("diskqueue(%s) failed to retrieveMetaData - %s", d.name, err)
	}

	_, ok := queue.GetConsumerConfigsByQueueID(d.name)
	if ok {
		d.consumerMode = true
	}

	go d.ioLoop()
	return &d
}

// Depth returns the depth of the queue
func (d *DiskBasedQueue) ReadContext() Context {
	ctx := Context{}
	ctx.WriteFileNum = d.writeSegmentNum
	ctx.WriteFile = d.GetFileName(ctx.WriteFileNum)
	return ctx
}

func (d *DiskBasedQueue) LatestOffset() queue.Offset {
	return queue.NewOffset(d.writeSegmentNum, d.writePos)
}

func (d *DiskBasedQueue) Depth() int64 {
	depth, ok := <-d.depthChan
	if !ok {
		// ioLoop exited
		depth = d.depth
	}
	return depth
}

// ReadChan returns the receive-only []byte channel for reading data
func (d *DiskBasedQueue) ReadChan() <-chan []byte {
	return d.readChan
}

// Put writes a []byte to the queue
func (d *DiskBasedQueue) Put(data []byte) WriteResponse {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.cfg.WriteTimeoutInMS)*time.Millisecond)
	defer cancel()

	size:=int64(len(data))
	stats.IncrementBy("disk_queue","inflight_data_size", size)

	d.RLock()
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error on put disk_queue,", v)
			}
		}
		d.RUnlock()
		defer stats.IncrementBy("disk_queue","inflight_data_size", size*-1)
	}()

	res:=WriteResponse{}
	if d.exitFlag == 1 {
		log.Errorf("queue [%v] exiting, data maybe lost", d.name)
		res.Error=errors.New("exiting")
		return res
	}

	if preventRead {
		err := checkCapacity(d.cfg)
		if err != nil {
			if rate.GetRateLimiterPerSecond(d.name, "disk_full_failure", 1).Allow() {
				log.Errorf("queue [%v] is readonly, %v", d.name, err)
			}
		}
		res.Error= errors.New("readonly")
		return res
	}

	select {
	case d.writeChan <- data:
		return <-d.writeResponseChan
	case <-ctx.Done():
		// Handle timeout
		res.Error = ctx.Err()
		return res
	}
}

// Close cleans up the queue and persists metadata
func (d *DiskBasedQueue) Close() error {
	err := d.exit(false)
	if err != nil {
		return err
	}
	return d.sync()
}

// Destroy cleans up all data for the specified queue
func (d *DiskBasedQueue) Destroy() error {
	err := d.Close()
	if err != nil {
		log.Errorf("failed to close queue [%v], err: %v", d.name, err)
		return err
	}
	if d.name == "" {
		log.Errorf("invalid queue name")
		return nil
	}
	dataPath := GetDataPath(d.name)
	err = os.RemoveAll(dataPath)
	if err != nil {
		log.Errorf("failed to delete queue [%v] path [%v], err: %v", d.name, dataPath, err)
		return err
	}
	return nil
}

func (d *DiskBasedQueue) Delete() error {
	return d.exit(true)
}

func (d *DiskBasedQueue) exit(deleted bool) error {
	d.Lock()

	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error on exit disk_queue,", v)
			}
		}
		d.Unlock()
	}()


	d.exitFlag = 1

	if deleted {
		log.Tracef("disk_queue(%s): deleting", d.name)
	} else {
		log.Tracef("disk_queue(%s): closing", d.name)
	}

	close(d.exitChan)
	// ensure that ioLoop has exited
	<-d.exitSyncChan

	close(d.depthChan)

	if d.readFile != nil {
		d.readFile.Close()
		d.readFile = nil
	}

	if d.writeFile != nil {
		d.writeFile.Close()
		d.writeFile = nil
	}

	return nil
}

// Empty destructively clears out any pending data in the queue
// by fast forwarding read positions and removing intermediate files
func (d *DiskBasedQueue) Empty() error {
	d.RLock()
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error on empty disk_queue,", v)
			}
		}
		d.RUnlock()
	}()

	if d.exitFlag == 1 {
		return errors.New("exiting")
	}

	log.Tracef("disk_queue(%s): emptying", d.name)

	d.emptyChan <- 1
	return <-d.emptyResponseChan
}

func (d *DiskBasedQueue) deleteAllFiles() error {
	err := d.skipToNextRWFile(true)

	innerErr := os.Remove(d.metaDataFileName())
	if innerErr != nil && !os.IsNotExist(innerErr) {
		log.Errorf("diskqueue(%s) failed to remove metadata file - %s", d.name, innerErr)
		return innerErr
	}

	return err
}

// 删除中间的错误文件，跳转到最后一个可写文件
func (d *DiskBasedQueue) skipToNextRWFile(delete bool) error {
	d.Lock()
	defer d.Unlock()

	var err error

	if d.readFile != nil {
		err = d.readFile.Close()
		if err != nil {
			panic(err)
		}
		d.readFile = nil
	}

	if d.writeFile != nil {
		d.writeFile.Sync()
		err = d.writeFile.Close()
		if err != nil {
			panic(err)
		}
		d.writeFile = nil
	}

	if delete{
		for i := d.readSegmentFileNum; i <= d.writeSegmentNum; i++ {

			//TODO, keep old files for a configure time window

			fn := d.GetFileName(i)
			//log.Error("delete:",fn)
			innerErr := os.Remove(fn)
			if innerErr != nil && !os.IsNotExist(innerErr) {
				log.Errorf("diskqueue(%s) failed to remove data file - %s", d.name, innerErr)
				err = innerErr
			}
		}
	}

	d.writeSegmentNum++
	d.writePos = 0
	d.readSegmentFileNum = d.writeSegmentNum
	d.readPos = 0
	d.nextReadFileNum = d.writeSegmentNum
	d.nextReadPos = 0
	d.depth = 0

	return err
}

// readOne performs a low level filesystem read for a single []byte
// while advancing read positions and rolling files, if necessary
func (d *DiskBasedQueue) readOne() ([]byte, error) {
	var err error
	var msgSize int32

	if d.readFile == nil {
		curFileName := d.GetFileName(d.readSegmentFileNum)

		//TODO if the file was compressed, decompress it first, and decompress few files ahead, keep # files decompressed

		//if !util.FileExists(curFileName){
		//	//log.Error("file not exists:",curFileName)
		//	return nil, errors.Errorf("file [%v] not exists",curFileName)
		//}

		d.readFile, err = os.OpenFile(curFileName, os.O_RDONLY, 0600)
		if err != nil {
			return nil, err
		}

		if global.Env().IsDebug {
			log.Tracef("disk_queue(%s): readOne() opened %s", d.name, curFileName)
		}

		if d.readPos > 0 {
			_, err = d.readFile.Seek(d.readPos, 0)
			if err != nil {
				d.readFile.Close()
				d.readFile = nil
				return nil, err
			}
		}

		// for "complete" files (i.e. not the "current" file), maxBytesPerFileRead
		// should be initialized to the file's size, or default to maxBytesPerFile
		d.maxBytesPerFileRead = d.cfg.MaxBytesPerFile
		if d.readSegmentFileNum < d.writeSegmentNum {
			stat, err := d.readFile.Stat()
			if err == nil {
				d.maxBytesPerFileRead = stat.Size()
			}
		}

		d.reader = bufio.NewReader(d.readFile)
	}

	err = binary.Read(d.reader, binary.BigEndian, &msgSize)
	if err != nil {
		d.readFile.Close()
		d.readFile = nil
		return nil, err
	}

	if msgSize < d.cfg.MinMsgSize || msgSize > d.cfg.MaxMsgSize {
		// this file is corrupt and we have no reasonable guarantee on
		// where a new message should begin
		d.readFile.Close()
		d.readFile = nil
		return nil, fmt.Errorf("invalid message read size (%d)", msgSize)
	}

	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(d.reader, readBuf)
	if err != nil {
		d.readFile.Close()
		d.readFile = nil
		return nil, err
	}

	totalBytes := int64(4 + msgSize)

	//log.Error("position:",d.readSegmentFileNum,",",d.readPos,",",totalBytes)

	// we only advance next* because we have not yet sent this to consumers
	// (where readSegmentFileNum, readPos will actually be advanced)
	d.nextReadPos = d.readPos + totalBytes
	d.nextReadFileNum = d.readSegmentFileNum

	// we only consider rotating if we're reading a "complete" file
	// and since we cannot know the size at which it was rotated, we
	// rely on maxBytesPerFileRead rather than maxBytesPerFile
	if d.readSegmentFileNum < d.writeSegmentNum && d.nextReadPos >= d.maxBytesPerFileRead {
		if d.readFile != nil {
			d.readFile.Close()
			d.readFile = nil
		}

		d.nextReadFileNum++
		d.nextReadPos = 0
	}

	if d.cfg.Compress.Message.Enabled {
		if global.Env().IsDebug {
			log.Tracef("decompress message: %v %v", d.readSegmentFileNum, d.readPos)
		}
		newData, err := zstd.ZSTDDecompress(nil, readBuf)
		if err != nil {
			return nil, err
		}
		return newData, nil
	}

	return readBuf, nil
}

type WriteResponse struct {
	Segment int64
	Position int64
	Error error
}

// writeOne performs a low level filesystem write for a single []byte
// while advancing write positions and rolling files, if necessary
func (d *DiskBasedQueue) writeOne(data []byte) WriteResponse {
	var err error
	var res WriteResponse

	if d.writeFile == nil {
		curFileName := d.GetFileName(d.writeSegmentNum)
		d.writeFile, err = os.OpenFile(curFileName, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			res.Error=err
			return res
		}

		log.Tracef("disk_queue(%s): writeOne() opened %s", d.name, curFileName)

		if d.writePos > 0 {
			_, err = d.writeFile.Seek(d.writePos, 0)
			if err != nil {
				d.writeFile.Close()
				d.writeFile = nil
				res.Error=err
				return res
			}
		}
	}

	//compress data
	if d.cfg.Compress.Message.Enabled {
		if global.Env().IsDebug {
			log.Tracef("compress message: %v %v", d.readSegmentFileNum, d.readPos)
		}
		newData, err := zstd.ZSTDCompress(nil, data, d.cfg.Compress.Message.Level)
		if err != nil {
			res.Error=err
			return res
		}
		data = newData
	}

	dataLen := int32(len(data))

	if dataLen < d.cfg.MinMsgSize || dataLen > d.cfg.MaxMsgSize {
		res.Error= fmt.Errorf("invalid message write size (%d) minMsgSize=%d maxMsgSize=%d", dataLen, d.cfg.MinMsgSize, d.cfg.MaxMsgSize)
		return res
	}

	d.writeBuf.Reset()
	err = binary.Write(&d.writeBuf, binary.BigEndian, dataLen)
	if err != nil {
		res.Error=err
		return res
	}

	_, err = d.writeBuf.Write(data)
	if err != nil {
		res.Error=err
		return res
	}

	// only write to the file once
	_, err = d.writeFile.Write(d.writeBuf.Bytes())
	if err != nil {
		d.writeFile.Close()
		d.writeFile = nil
		res.Error=err
		return res
	}

	totalBytes := int64(4 + dataLen)
	d.writePos += totalBytes
	d.depth += 1

	if d.writePos >= d.cfg.MaxBytesPerFile {
		if d.readSegmentFileNum == d.writeSegmentNum {
			d.maxBytesPerFileRead = d.writePos
		}

		//notify listener that we are writing to a new file
		Notify(d.name, WriteComplete, d.writeSegmentNum)

		d.writeSegmentNum++
		d.writePos = 0

		// sync every time we start writing to a new file
		err = d.sync()
		if err != nil {
			log.Errorf("diskqueue(%s) failed to sync - %s", d.name, err)
		}

		if d.writeFile != nil {
			d.writeFile.Close()
			d.writeFile = nil
		}
	}

	res.Error=err
	res.Segment=d.writeSegmentNum
	res.Position=d.writePos
	return res
}

// sync fsyncs the current writeFile and persists metadata
func (d *DiskBasedQueue) sync() error {
	if d.writeFile != nil {
		err := d.writeFile.Sync()
		if err != nil {
			d.writeFile.Close()
			d.writeFile = nil
			return err
		}
	}

	err := d.persistMetaData()
	if err != nil {
		return err
	}

	d.needSync = false
	return nil
}

// retrieveMetaData initializes state from the filesystem
func (d *DiskBasedQueue) retrieveMetaData() error {
	var f *os.File
	var err error

	fileName := d.metaDataFileName()
	f, err = os.OpenFile(fileName, os.O_RDONLY, 0600)
	if f != nil {
		defer f.Close()
	}

	if err != nil {
		return err
	}

	var depth int64
	_, err = fmt.Fscanf(f, "%d\n%d,%d\n%d,%d\n",
		&depth,
		&d.readSegmentFileNum, &d.readPos,
		&d.writeSegmentNum, &d.writePos)
	if err != nil {
		return err
	}
	d.depth = depth
	d.nextReadFileNum = d.readSegmentFileNum
	d.nextReadPos = d.readPos

	return nil
}

// persistMetaData atomically writes state to the filesystem
func (d *DiskBasedQueue) persistMetaData() error {
	d.metaLock.Lock()
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error on persist disk_queue metadata,", v)
			}
		}
		d.metaLock.Unlock()
	}()

	var f *os.File
	var err error

	fileName := d.metaDataFileName()
	tmpFileName := fmt.Sprintf("%s.%d.tmp", fileName, rand.Int())

	// write to tmp file
	f, err = os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		if f != nil {
			f.Close()
		}
		return err
	}

	_, err = fmt.Fprintf(f, "%d\n%d,%d\n%d,%d\n",
		d.depth,
		d.readSegmentFileNum, d.readPos,
		d.writeSegmentNum, d.writePos)
	if err != nil {
		f.Close()
		return err
	}
	f.Sync()
	f.Close()

	// atomically rename
	return util.AtomicFileRename(tmpFileName, fileName)
}

func (d *DiskBasedQueue) metaDataFileName() string {
	return path.Join(d.dataPath, "meta.dat")
}

func (d *DiskBasedQueue) GetFileName(segmentID int64) string {
	return GetFileName(d.name, segmentID)
}

func (d *DiskBasedQueue) checkTailCorruption(depth int64) {
	if d.readSegmentFileNum < d.writeSegmentNum || d.readPos < d.writePos {
		return
	}

	// we've reached the end of the diskqueue
	// if depth isn't 0 something went wrong
	if depth != 0 {
		if depth < 0 {
			log.Errorf(
				"diskqueue(%s) negative depth at tail (%d), metadata corruption, resetting 0...",
				d.name, depth)
		} else if depth > 0 {
			log.Errorf(
				"diskqueue(%s) positive depth at tail (%d), data loss, resetting 0...",
				d.name, depth)
		}
		// force set depth 0
		d.depth = 0
		d.needSync = true
	}

	if d.readSegmentFileNum != d.writeSegmentNum || d.readPos != d.writePos {

		if d.readSegmentFileNum > d.writeSegmentNum {
			log.Errorf(
				"diskqueue(%s) readSegmentFileNum > writeSegmentNum (%d > %d), corruption, skipping to next writeSegmentNum and resetting 0...",
				d.name, d.readSegmentFileNum, d.writeSegmentNum)
		}

		if d.readPos > d.writePos {
			log.Errorf(
				"diskqueue(%s) readPos > writePos (%d > %d), corruption, skipping to next writeSegmentNum and resetting 0...",
				d.name, d.readPos, d.writePos)
		}

		d.skipToNextRWFile(true)
		d.needSync = true
	}
}

func (d *DiskBasedQueue) readMoveForward() {
	oldReadFileNum := d.readSegmentFileNum
	d.readSegmentFileNum = d.nextReadFileNum
	d.readPos = d.nextReadPos
	d.depth -= 1

	// see if we need to clean up the old file
	if oldReadFileNum != d.nextReadFileNum {
		// sync every time we start reading from a new file
		d.needSync = true

		if global.Env().IsDebug {
			log.Tracef("queue:%v old file:%v, new file:%v", d.name, oldReadFileNum, d.nextReadFileNum)
		}

		consumers, ok := queue.GetConsumerConfigsByQueueID(d.name)
		if !ok || len(consumers) == 0 {
			fn := d.GetFileName(oldReadFileNum)
			if util.FileExists(fn) {
				if global.Env().IsDebug {
					log.Debugf("queue:%v delete old file:%v, new file:%v", d.name, oldReadFileNum, d.nextReadFileNum)
				}
				err := os.Remove(fn)
				if err != nil {
					log.Errorf("failed to Remove(%s) - %s", fn, err)
				}
			}
		}
	}

	d.checkTailCorruption(d.depth)
}

func (d *DiskBasedQueue) handleReadError() {
	// jump to the next read file and rename the current (bad) file
	if d.readSegmentFileNum == d.writeSegmentNum {
		// if you can't properly read from the current write file it's safe to
		// assume that something is fucked and we should skip the current file too
		if d.writeFile != nil {
			d.writeFile.Close()
			d.writeFile = nil
		}
		d.writeSegmentNum++
		d.writePos = 0
	}

	//skip queue with consumers
	_, ok := queue.GetConsumerConfigsByQueueID(d.name)
	if ok {
		if !d.consumerMode {
			d.consumerMode = true
		}
		//Consumer mode, the first file is deleted, no need fetch to read channel
		//d.readSegmentFileNum=d.writeSegmentNum
		//d.readPos=d.writePos
		return
	}

	badFn := d.GetFileName(d.readSegmentFileNum)

	if util.FileExists(badFn) {

		badRenameFn := badFn + ".bad"
		log.Warnf(
			"diskqueue(%s) jump to next file and saving bad file as %s",
			d.name, badRenameFn)

		err := util.AtomicFileRename(badFn, badRenameFn)
		if err != nil {
			log.Errorf(
				"diskqueue(%s) failed to rename bad diskqueue file %s to %s",
				d.name, badFn, badRenameFn)
		}
	}

	d.readSegmentFileNum++
	d.readPos = 0
	d.nextReadFileNum = d.readSegmentFileNum
	d.nextReadPos = 0

	// significant state change, schedule a sync on the next iteration
	d.needSync = true
}

// ioLoop provides the backend for exposing a go channel (via ReadChan())
// in support of multiple concurrent queue consumers
//
// it works by looping and branching based on whether or not the queue has data
// to read and blocking until data is either read or written over the appropriate
// go channels
//
// conveniently this also means that we're asynchronously reading from the filesystem
func (d *DiskBasedQueue) ioLoop() {

	var dataRead []byte
	var err error
	var count int64
	var r chan []byte

	syncTicker := time.NewTicker(time.Duration(d.cfg.SyncTimeoutInMS) * time.Millisecond)

	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error to disk_queue ioLoop,", v)
			}
			syncTicker.Stop()
			d.exitSyncChan <- 1
			return
		}
	}()

	for {
		// dont sync all the time :)
		if count == d.cfg.SyncEveryRecords {
			d.needSync = true
		}

		if d.needSync {
			err = d.sync()
			if err != nil {
				log.Errorf("diskqueue(%s) failed to sync - %s", d.name, err)
			}
			count = 0
		}
		if !d.consumerMode && ((d.readSegmentFileNum < d.writeSegmentNum) || (d.readPos < d.writePos)) {
			if d.nextReadPos == d.readPos {
				dataRead, err = d.readOne()
				if err != nil {
					d.handleReadError()
					time.Sleep(1 * time.Second)
					continue
				}
			}
			r = d.readChan
		} else {
			r = nil
		}

		select {
		// the Go channel spec dictates that nil channel operations (read or write)
		// in a select are skipped, we set r to d.readChan only when there is data to read
		case r <- dataRead:
			count++
			// readMoveForward sets needSync flag if a file is removed
			d.readMoveForward()
		case d.depthChan <- d.depth:
		case <-d.emptyChan:
			d.emptyResponseChan <- d.deleteAllFiles()
			count = 0
		case dataWrite := <-d.writeChan:
			count++
			d.writeResponseChan <- d.writeOne(dataWrite)
		case <-syncTicker.C:
			if count == 0 {
				// avoid sync when there's no activity
				continue
			}
			d.needSync = true
		case <-d.exitChan:
			goto exit
		}
	}

exit:
	log.Tracef("disk_queue(%s): closing ... ioLoop", d.name)
	syncTicker.Stop()
	d.exitSyncChan <- 1
}

func (d *DiskBasedQueue) UpdateSegmentConsumerInReading(consumerID string, segment int64) {
	d.consumersInReading.Store(consumerID, segment)
}

func (d *DiskBasedQueue) DeleteSegmentConsumerInReading(consumerID string) {
	d.consumersInReading.Delete(consumerID)
}
