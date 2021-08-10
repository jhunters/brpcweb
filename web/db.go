/*
 * @Author: Malin Xie
 * @Description:
 * @Date: 2021-07-26 14:25:51
 */
package web

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// LoadData load all data from bbolt db by pagination supports
func LoadData(db *bolt.DB, bucketName string, page, pagesize int) (map[string][]byte, int64, error) {
	data := make(map[string][]byte)

	tx, err := db.Begin(true)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Commit()
	bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
	if err != nil {
		return nil, 0, err
	}

	begin := (page-1)*pagesize + 1
	var end int
	if pagesize < 0 {
		end = bucket.Stats().KeyN
	} else {
		end = page * pagesize
	}
	var i int = 1

	cursor := bucket.Cursor()
	k, v := cursor.First()
	for k != nil {
		if i > end {
			break
		}
		if i >= begin && i <= end {
			data[string(k)] = v
		}
		k, v = cursor.Next()
		i++
	}

	return data, int64(bucket.Stats().KeyN), err
}

// getData
func getData(db *bolt.DB, name, bucketName string) ([]byte, error) {
	tx, err := db.Begin(true)
	if err != nil {
		return nil, err
	}
	defer tx.Commit()
	bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
	if err != nil {
		return nil, err
	}
	v := bucket.Get([]byte(name))
	return v, nil
}

// putData
func putData(db *bolt.DB, name, bucketName string, data []byte, errExist bool) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		old := bucket.Get([]byte(name))
		if old != nil && errExist {
			return fmt.Errorf("key '%s' aleady exist", name)
		}

		return bucket.Put([]byte(name), data)
	})

	return err
}

// deleteData
func deleteData(db *bolt.DB, name, bucketName string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		err = bucket.Delete([]byte(name))
		return err
	})

	return err
}
