package main

import (
	"encoding/json"
	"hash/fnv"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/oklog/ulid"
	"github.com/pkg/errors"
)

var Clock = time.Now

// RunState describe configuration key generated at runtime we do not want to change when a restart occurs
type RunState struct {
	ID uint64 `json:"id"`
}

func loadID(datadir string) (uint64, error) {
	state := RunState{}
	filename := path.Join(datadir, "run_state.json")

	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		if os.IsNotExist(err) {
			state, err = initRunState(datadir)
			if err != nil {
				return 0, errors.Wrap(err, "failed to create run state")
			}
			return state.ID, nil
		}
		return 0, err
	}
	err = json.NewDecoder(fd).Decode(&state)
	if err != nil {
		return 0, err
	}
	return state.ID, nil
}

func saveRunState(state RunState, datadir string) error {
	fd, err := os.Create(path.Join(datadir, "run_state.json"))
	if err != nil {
		return err
	}
	defer fd.Close()
	return json.NewEncoder(fd).Encode(&state)
}

func hashID(id string) uint64 {
	h := fnv.New64()
	h.Write([]byte(id))
	return h.Sum64()
}

func initRunState(datadir string) (RunState, error) {
	t := Clock().UTC()
	entropy := rand.New(rand.NewSource(t.UnixNano()))
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		return RunState{}, err
	}
	state := RunState{ID: hashID(id.String())}
	return state, saveRunState(state, datadir)
}
