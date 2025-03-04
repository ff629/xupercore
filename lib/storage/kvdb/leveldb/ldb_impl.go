package leveldb

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/xuperchain/xupercore/lib/metrics"
	"github.com/xuperchain/xupercore/lib/storage/kvdb"
)

var (
	//ldbp = pprof.NewProfile("ldb")
	//pcounter int64
)

// LDBDatabase define data structure of storage
type LDBDatabase struct {
	fn string      // filename of db
	db *leveldb.DB // LevelDB instance
}

// GetInstance get instance of LDBDatabase
func NewKVDBInstance(param *kvdb.KVParameter) (kvdb.Database, error) {
	baseDB := new(LDBDatabase)
	err := baseDB.Open(param.GetDBPath(), map[string]interface{}{
		"cache":       param.GetMemCacheSize(),
		"fds":         param.GetFileHandlersCacheSize(),
		"dataPaths":   param.GetOtherPaths(),
		"storageType": param.GetStorageType(),
	})
	if err != nil {
		return nil, err
	}

	return baseDB, nil
}

func init() {
	kvdb.Register("leveldb", NewKVDBInstance)
}

func setDefaultOptions(options map[string]interface{}) {
	if options["cache"] == nil {
		options["cache"] = 16
	}
	if options["fds"] == nil {
		options["fds"] = 16
	}
	if options["dataPaths"] == nil {
		options["dataPaths"] = []string{}
	}
}

func (db *LDBDatabase) Open(path string, options map[string]interface{}) error {
	setDefaultOptions(options)
	if _, ok := options["storageType"]; !ok {
		return fmt.Errorf("open database fail. err:empty storageType")
	}
	switch options["storageType"] {
	case kvdb.StorageTypeSingle:
		return db.OpenSingle(path, options)
	case kvdb.StorageTypeMulti:
		return db.OpenMulti(path, options)
	case kvdb.StorageTypeCloud:
		return db.OpenCloud(path, options)
	default:
		return fmt.Errorf("open database fail. err:invalid storageType:%s", options["storageType"])
	}
}

// Path returns the path to the database directory.
func (db *LDBDatabase) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *LDBDatabase) Put(key []byte, value []byte) error {
	// Generate the data to write to disk, update the meter and write
	//value = rle.Compress(value)
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "Put", "OK").Inc()
	metrics.BytesCounter.WithLabelValues("levelDB", "Put", "key").Add(float64(len(key)))
	metrics.BytesCounter.WithLabelValues("levelDB", "Put", "value").Add(float64(len(value)))
	return db.db.Put(key, value, nil)
}

// Has if the given key exists
func (db *LDBDatabase) Has(key []byte) (bool, error) {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "Has", "OK").Inc()
	return db.db.Has(key, nil)
}

// Get returns the given key if it's present.
func (db *LDBDatabase) Get(key []byte) ([]byte, error) {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "Get", "OK").Inc()
	// Retrieve the key and increment the miss counter if not found
	dat, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// Delete deletes the key from the queue and database
func (db *LDBDatabase) Delete(key []byte) error {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "Delete", "OK").Inc()
	// Execute the actual operation
	return db.db.Delete(key, nil)
}

// NewIterator returns an instance of Iterator
func (db *LDBDatabase) NewIterator() kvdb.Iterator {
	return db.db.NewIterator(nil, nil)
}

// NewIteratorWithRange returns an instance of Iterator with range
func (db *LDBDatabase) NewIteratorWithRange(start []byte, limit []byte) kvdb.Iterator {
	keyRange := &util.Range{Start: start, Limit: limit}
	return db.db.NewIterator(keyRange, nil)
}

// NewIteratorWithPrefix returns an instance of Iterator with prefix
func (db *LDBDatabase) NewIteratorWithPrefix(prefix []byte) kvdb.Iterator {
	return db.db.NewIterator(util.BytesPrefix(prefix), nil)
}

// Close close database instance
func (db *LDBDatabase) Close() {
	db.db.Close()
}

// LDB returns ldb instance
func (db *LDBDatabase) LDB() *leveldb.DB {
	return db.db
}

// NewBatch returns batch instance of ldb
func (db *LDBDatabase) NewBatch() kvdb.Batch {
	return &ldbBatch{db: db.db, b: new(leveldb.Batch), keys: map[string]bool{}}
}

type ldbBatch struct {
	db   *leveldb.DB
	b    *leveldb.Batch
	size int
	keys map[string]bool
}

func (b *ldbBatch) Put(key, value []byte) error {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "BatchPut", "OK").Inc()
	metrics.BytesCounter.WithLabelValues("levelDB", "BatchPut", "key").Add(float64(len(key)))
	metrics.BytesCounter.WithLabelValues("levelDB", "BatchPut", "value").Add(float64(len(value)))
	b.b.Put(key, value)
	b.size += len(value)
	return nil
}

func (b *ldbBatch) Delete(key []byte) error {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "BatchDelete", "OK").Inc()
	b.b.Delete(key)
	b.size += len(key)
	return nil
}

func (b *ldbBatch) PutIfAbsent(key, value []byte) error {
	if !b.keys[string(key)] {
		b.b.Put(key, value)
		b.size += len(value)
		b.keys[string(key)] = true
		return nil
	}
	return fmt.Errorf("duplicated key in batch, (HEX) %x", key)
}

func (b *ldbBatch) Exist(key []byte) bool {
	metrics.CallMethodCounter.WithLabelValues("levelDB", "BatchExist", "OK").Inc()
	return b.keys[string(key)]
}

func (b *ldbBatch) Write() error {
	//ldbp.Add(atomic.AddInt64(&pcounter, 1), 1)
	metrics.CallMethodCounter.WithLabelValues("levelDB", "BatchWrite", "OK").Inc()
	return b.db.Write(b.b, nil)
}

func (b *ldbBatch) ValueSize() int {
	return b.size
}

func (b *ldbBatch) Reset() {
	b.b.Reset()
	b.size = 0
	b.keys = map[string]bool{}
}
