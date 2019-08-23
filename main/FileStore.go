package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/tulir/mautrix-go"
)

type FileStore struct {
	path string

	FilterID  string                   `json:"filter_id"`
	NextBatch string                   `json:"next_batch"`
	Rooms     map[string]*mautrix.Room `json:"-"`
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:  path,
		Rooms: make(map[string]*mautrix.Room),
	}
}

func (fs *FileStore) Save() error {
	data, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fs.path, data, 0600)
	return err
}

func (fs *FileStore) Load() error {
	data, err := ioutil.ReadFile(fs.path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, fs)
	return err
}

func (fs *FileStore) SaveFilterID(_, filterID string) {
	fs.FilterID = filterID
	fs.Save()
}

func (fs *FileStore) LoadFilterID(_ string) string {
	return fs.FilterID
}

func (fs *FileStore) SaveNextBatch(_, nextBatchToken string) {
	fs.NextBatch = nextBatchToken
	fs.Save()
}

func (fs *FileStore) LoadNextBatch(_ string) string {
	return fs.NextBatch
}

func (fs *FileStore) SaveRoom(room *mautrix.Room) {
	fs.Rooms[room.ID] = room
}

func (fs *FileStore) LoadRoom(roomID string) *mautrix.Room {
	return fs.Rooms[roomID]
}
