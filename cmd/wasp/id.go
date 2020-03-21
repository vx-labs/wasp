package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/oklog/ulid"
)

var Clock = time.Now

// RunState describe configuration key generated at runtime we do not want to change when a restart occurs
type RunState struct {
	ID string `json:"id"`
}

func loadID(datadir string) (string, error) {
	state := RunState{}
	filename := path.Join(datadir, "run_state.json")

	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		if os.IsNotExist(err) {
			state, err = initRunState(datadir)
			return state.ID, nil
		}
		return "", err

	}
	err = json.NewDecoder(fd).Decode(&state)
	if err != nil {
		return "", err
	}
	return state.ID, nil
}

func saveRunState(state RunState, datadir string) error {
	fd, err := os.Create(path.Join(datadir, "run_state.json"))
	defer fd.Close()
	if err != nil {
		return err
	}
	return json.NewEncoder(fd).Encode(&state)
}

func initRunState(datadir string) (RunState, error) {
	t := Clock().UTC()
	entropy := rand.New(rand.NewSource(t.UnixNano()))
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		return RunState{}, err
	}
	state := RunState{ID: id.String()}
	return state, saveRunState(state, datadir)
}
