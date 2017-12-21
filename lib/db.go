package main

import (
	"encoding/json"
	"log"

	"github.com/garyburd/redigo/redis"
)

// DB used to handle share submission.
type DB struct {
	pool *redis.Pool

	SubmitChan chan Share
}

// A Share submitted to redis.
type Share struct {
	Submitter     string
	Difficulty    float64
	NetDifficulty float64
	Subsidy       float64
	Host          string
	Server        string
	Valid         bool
}

func NewDB(redisHost, redisPass string) (*DB, error) {
	pool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", redisHost)
		if err != nil {
			return nil, err
		}

		if redisPass != "" {
			if _, err := c.Do("AUTH", redisPass); err != nil {
				c.Close()
				return nil, err
			}
		}

		return c, err
	}, 10)

	conn := pool.Get()
	defer conn.Close()

	if _, err := conn.Do("PING"); err != nil {
		return nil, err
	}

	db := DB{
		pool:       pool,
		SubmitChan: make(chan Share, 1024),
	}

	// run a loop to publish share
	go db.serve()

	return &db, nil
}

func (db *DB) serve() {
	for share := range db.SubmitChan {
		func() {
			conn := db.pool.Get()
			defer conn.Close()

			data, _ := json.Marshal(share)

			_, err := conn.Do("PUBLISH", "shares", data)
			if err != nil {
				log.Println("[db]Could not publish share: " + string(data))
			}
		}()
	}
}
