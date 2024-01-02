package fsmHashmap

import (
	"encoding/json"
	"fmt"
	"github.com/cornelk/hashmap"
	"io"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
)

type FsmHashmap struct {
	DataBase database
}

func NewFsmHashmap() *FsmHashmap {
	fsm := &FsmHashmap{
		DataBase: NewDatabase(),
	}
	return fsm
}

func (f *FsmHashmap) Apply(l *raft.Log) interface{} {
	fmt.Println("apply data:", string(l.Data))
	data := strings.Split(string(l.Data), ",")
	op := data[0]
	if op == "hashmap" {
		key := data[1]
		value := data[2]
		f.DataBase.SetHashmap(key, value)
	}

	return nil
}

func (f *FsmHashmap) Snapshot() (raft.FSMSnapshot, error) {
	return &f.DataBase, nil
}

func (f *FsmHashmap) Restore(io.ReadCloser) error {
	return nil
}

type database struct {
	Data *hashmap.Map[string, string]
	mu   sync.Mutex
}

func NewDatabase() database {
	return database{
		Data: hashmap.New[string, string](),
	}
}

func (d *database) GetHashmap(key string) string {
	d.mu.Lock()
	value, _ := d.Data.Get(key)
	d.mu.Unlock()
	return value
}

func (d *database) SetHashmap(key, value string) {
	d.mu.Lock()
	d.Data.Set(key, value)
	d.mu.Unlock()
}

func (d *database) Persist(sink raft.SnapshotSink) error {
	d.mu.Lock()
	data, err := json.Marshal(d.Data)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	sink.Write(data)
	sink.Close()
	return nil
}

func (d *database) Release() {}
