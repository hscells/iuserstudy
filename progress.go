package main

import "github.com/boltdb/bolt"

type progress []byte
type status int

// 1. Pre-Experiment Questionnaire
// 2. Pre-Task Questionnaire
// 3. Interface 1 & Protocol 1
// 4. Post-Task Questionnaire
// 5. Pre-Task Questionnaire
// 6. Interface 2 & Protocol 2
// 7. Post-Task Questionnaire
// 8. Post-Experiment Questionnaire
const tasks = 8

const (
	complete status = iota
	incomplete
)

func newProgress() *progress {
	p := make(progress, 1)
	return &p
}

func (p *progress) Step(user []byte, db *bolt.DB) {
	[]byte(*p)[0]++
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketProgress))
		b.Put(user, *p)
	})
}
