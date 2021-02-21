package main

import (
	"encoding/json"
	"io/ioutil"

	"maunium.net/go/mautrix"
)

//FileStore required by the bridgeAPI
type FileStore struct {
	path string

	FilterID  string                   `json:"filter_id"`
	NextBatch string                   `json:"next_batch"`
	Rooms     map[string]*mautrix.Room `json:"-"`
}

//NewFileStore creates a new filestore
func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:  path,
		Rooms: make(map[string]*mautrix.Room),
	}
}

//Save saves the store
func (fs *FileStore) Save() error {
	data, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fs.path, data, 0600)
	return err
}

//Load loads the store
func (fs *FileStore) Load() error {
	data, err := ioutil.ReadFile(fs.path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, fs)
	return err
}

//SaveFilterID sets filterID and saves
func (fs *FileStore) SaveFilterID(_, filterID string) {
	fs.FilterID = filterID
	fs.Save()
}

//LoadFilterID loadsFilterID
func (fs *FileStore) LoadFilterID(_ string) string {
	return fs.FilterID
}

//SaveNextBatch saves Next batch
func (fs *FileStore) SaveNextBatch(_, nextBatchToken string) {
	fs.NextBatch = nextBatchToken
	fs.Save()
}

//LoadNextBatch loads  next batch
func (fs *FileStore) LoadNextBatch(_ string) string {
	return fs.NextBatch
}

//SaveRoom saves room
func (fs *FileStore) SaveRoom(room *mautrix.Room) {
	fs.Rooms[string(room.ID)] = room
}

//LoadRoom loads room
func (fs *FileStore) LoadRoom(roomID string) *mautrix.Room {
	return fs.Rooms[roomID]
}
