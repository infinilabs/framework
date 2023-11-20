/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package simple_kv

import (
	"bufio"
	"bytes"
	"encoding/json"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

// KVStore represents a simple key-value store.
type KVStore struct {
	data     map[string][]byte
	wal      *WAL
	mu       sync.Mutex
	filename string
}

// LastState represents the last state of the key-value store.
type LastState struct {
	Data map[string][]byte `json:"data"`
}

// WAL represents a Write-Ahead Log for storing key-value changes.
type WAL struct {
	filename string
	mu       sync.Mutex
	walFile  *os.File
}

// NewKVStore creates a new key-value store and initializes the current state from the last state file.
func NewKVStore(lastStateFilename, walFilename string) *KVStore {
	kv := &KVStore{
		data:     make(map[string][]byte),
		wal:      &WAL{filename: walFilename},
		filename: lastStateFilename,
	}
	kv.loadFromLastState()
	kv.loadFromWAL()

	kv.wal.Open()

	global.RegisterShutdownCallback(func() {
		kv.wal.Close()
	})

	global.RegisterBackgroundCallback(&global.BackgroundTask{Tag: "simple_kv", Interval: time.Second * 10, Func: func() {
		kv.periodicSaveState()
	},
	})

	return kv
}

func (wal *WAL) Open() error {
	var err error
	wal.walFile, err = os.OpenFile(wal.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	return err
}

func (wal *WAL) Close() {
	wal.walFile.Close()
}

func (kv *KVStore) Set(key string, value []byte) error {
	//log.Error("set key: ", key, " value: ", string(value))

	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[key] = value
	if err := kv.wal.writeEntry(key, value); err != nil {
		return err
	}
	return nil
}

// Delete removes a key-value pair from the store and writes to the WAL synchronously.
func (kv *KVStore) Delete(key string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.data, key)

	if err := kv.wal.writeEntry(key, []byte("")); err != nil {
		return err
	}
	return nil
}

// Load the current state from the last state file.
func (kv *KVStore) loadFromLastState() {
	if _, err := os.Stat(kv.filename); err == nil {
		data, err := ioutil.ReadFile(kv.filename)
		if err != nil {
			log.Errorf("Error loading last state file: %v", err)
			return
		}
		var lastState LastState
		if err := json.Unmarshal(data, &lastState); err != nil {
			log.Errorf("Error decoding last state file: %v", err)
			return
		}
		kv.mu.Lock()
		defer kv.mu.Unlock()
		kv.data = lastState.Data
	}
}

const splitChar = "\t\t"

// Split a line into key and value.
func splitLine(line []byte) [][]byte {
	return bytes.Split(line, []byte(splitChar))
}

// LoadFromWAL loads the data from the WAL file and applies it to the store.
func (kv *KVStore) loadFromWAL() {
	kv.wal.mu.Lock()
	defer kv.wal.mu.Unlock()

	file, err := os.Open(kv.wal.filename)
	if err != nil {
		log.Errorf("Error opening WAL file: %v", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		parts := splitLine(line)
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			if len(value) == 0 {
				delete(kv.data, string(key))
			} else {
				kv.data[string(key)] = value
			}
		}
	}
}

// Write an entry to the WAL file.
func (wal *WAL) writeEntry(key string, value []byte) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	buffer := bytes.Buffer{}
	buffer.WriteString(key)
	buffer.WriteString(splitChar)
	buffer.Write(value)
	buffer.WriteString("\n")
	_, err := wal.walFile.Write(buffer.Bytes())
	wal.walFile.Sync()

	//log.Error("write wal:", buffer.String())
	return err
}

// Periodically save the current state to the last state file.
func (kv *KVStore) periodicSaveState() {
	kv.saveToLastState()
	kv.createNewWAL()
}

// Save the current state to the last state file in JSON format.
func (kv *KVStore) saveToLastState() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	lastState := LastState{Data: kv.data}
	data, err := json.Marshal(lastState)
	if err != nil {
		log.Errorf("Error marshaling last state to JSON: %v", err)
		return
	}

	if err := ioutil.WriteFile(kv.filename, data, 0644); err != nil {
		log.Errorf("Error saving last state: %v", err)
	}
}

// Create a new WAL file to store future changes.
func (kv *KVStore) createNewWAL() {
	kv.wal.mu.Lock()
	defer kv.wal.mu.Unlock()

	kv.wal.Close()
	if err := os.Rename(kv.wal.filename, kv.wal.filename+".bak"); err != nil {
		log.Errorf("Error renmae old WAL file: %v", err)
	}
	kv.wal.Open()
}

func (kv *KVStore) Get(key string) ([]byte, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	v, ok := kv.data[key]
	if !ok {
		return nil, nil
	}
	valCopy := append([]byte{}, v...)
	return valCopy, nil
}
