package main

import (
	"github.com/boltdb/bolt"
	"github.com/go-errors/errors"
)

type progress []byte
type status byte
type system byte
type protocol byte

const (
	queryvis system = 1
	pubmed   system = 2
)

const (
	p1 protocol = 1
	p2 protocol = 2
)

// 1. Pre-Experiment Questionnaire
// 2. Pre-Task Questionnaire
// 3. Interface 1 & Protocol 1
// 4. Post-Task Questionnaire
// 5. Pre-Task Questionnaire
// 6. Interface 2 & Protocol 2
// 7. Post-Task Questionnaire
// 8. Post-Experiment Questionnaire
// 9. Study Completed
const (
	preExperimentQuestionnaire status = iota + 1
	preTaskQuestionnaire1
	experiment1
	postTaskQuestionnaire1
	preTaskQuestionnaire2
	experiment2
	postTaskQuestionnaire2
	postExperimentQuestionnaire
	studyComplete
)

var steps = []status{
	preExperimentQuestionnaire,
	preTaskQuestionnaire1,
	experiment1,
	postTaskQuestionnaire1,
	preTaskQuestionnaire2,
	experiment2,
	postTaskQuestionnaire2,
	postExperimentQuestionnaire,
	studyComplete,
}

const protocols = 2
const complete = 1

func newProgress(db *bolt.DB) (progress, error) {
	var iv, pv byte
	err := db.Update(func(tx *bolt.Tx) error {
		// Get and update the interface.
		i := tx.Bucket([]byte(bucketInterface))
		b1 := i.Get([]byte(bucketInterface))
		iv = b1[0]
		if b1[0] == byte(queryvis) {
			b1 = []byte{byte(pubmed)}
		} else {
			b1 = []byte{byte(queryvis)}
		}
		err := i.Put([]byte(bucketInterface), b1)
		if err != nil {
			return err
		}

		// Get and update the protocol.
		// First byte contains the protocol,
		// second byte contains the number of assigned protocols so far.
		p := tx.Bucket([]byte(bucketProtocol)) // Protocol bucket.
		b2 := p.Get([]byte(bucketProtocol))    // Bytes for this key.
		pv = b2[0]                             // Which protocol?
		idx := b2[1]                           // Index of protocol assignment.
		if idx >= protocols*2 {
			b2 = []byte{byte(p2), idx}
		} else if b2[1] == 0 {
			b2 = []byte{byte(p1), idx}
		}
		// Note the increase of index.
		return i.Put([]byte(bucketProtocol), []byte{b2[0], idx + 1})
	})
	return progress{byte(preExperimentQuestionnaire), byte(iv), byte(pv)}, err
}

func (p progress) Get() (status, system, protocol) {
	return status(p[0]), system(p[1]), protocol(p[2])
}

func (p progress) Update(user []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketProgress))
		err := b.Put(user, p)
		return err
	})
}

func (p progress) Step(user []byte, db *bolt.DB) (progress, error) {
	// Do not progress if the experiments are complete.
	if status(p[0]) == studyComplete {
		return p, errors.New("cannot progress past completion of study")
	}

	_, i, prot := p.Get()
	// Update the interface and protocol for the next task.
	if status(p[0]) == experiment1 {
		if i == system(pubmed) {
			i = system(queryvis)
		} else {
			i = system(pubmed)
		}
		if prot == protocol(p1) {
			prot = protocol(p2)
		} else {
			prot = protocol(p1)
		}
	}

	pr := progress{byte(status(p[0] + 1)), byte(i), byte(prot)}

	// Then commit the step to the database.
	return pr, db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketProgress))
		err := b.Put(user, pr)
		return err
	})
}
