package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

//FileStore required by the bridgeAPI
type FileStore struct {
	path string

	FilterID    string                         `json:"filter_id"`
	NextBatch   string                         `json:"next_batch"`
	Rooms       map[id.RoomID]*mautrix.Room    `json:"rooms"`
	Memberships map[id.RoomID]event.Membership `json:"memberships"`
}

//NewFileStore creates a new filestore
func NewFileStore(path string) *FileStore {
	store := FileStore{
		path:  path,
		Rooms: make(map[id.RoomID]*mautrix.Room),
	}
	store.Load()
	fmt.Println("File: ", store)
	return &store
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
func (fs *FileStore) SaveFilterID(userID id.UserID, filterID string) {
	fs.FilterID = filterID
	fs.Save()
}

//LoadFilterID loadsFilterID
func (fs *FileStore) LoadFilterID(userID id.UserID) string {
	return fs.FilterID
}

//SaveNextBatch saves Next batch
func (fs *FileStore) SaveNextBatch(userID id.UserID, nextBatchToken string) {
	fs.NextBatch = nextBatchToken
	fs.Save()
}

//LoadNextBatch loads  next batch
func (fs *FileStore) LoadNextBatch(userID id.UserID) string {
	return fs.NextBatch
}

//SaveRoom saves room
func (fs *FileStore) SaveRoom(room *mautrix.Room) {
	fs.Rooms[room.ID] = room
	fs.Save()
}

//LoadRoom loads room
func (fs *FileStore) LoadRoom(roomID id.RoomID) *mautrix.Room {
	return fs.Rooms[roomID]
}
