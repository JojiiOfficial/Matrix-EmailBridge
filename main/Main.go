package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-imap/client"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"github.com/tulir/mautrix-go"
)

var matrixClient *mautrix.Client
var mailClient *client.Client
var db *sql.DB

func initDB() error {
	database, era := sql.Open("sqlite3", "./data.db")
	if era != nil {
		panic(era)
	}
	db = database
	return era
}

func initCfg() bool {
	viper.SetConfigType("json")
	viper.SetConfigFile("./cfg.json")
	viper.AddConfigPath("./")
	viper.SetConfigName("cfg")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("config not found. creating new one")
		viper.SetDefault("emailhost", "host.de")
		viper.SetDefault("port", "993")
		viper.SetDefault("username", "a@a.de")
		viper.SetDefault("password", "passs")
		viper.SetDefault("matrixServer", "matrix.org")
		viper.SetDefault("matrixaccesstoken", "hlaksdjhaslkfslkj")
		viper.SetDefault("matrixuserid", "@m:matrix.org")
		viper.SetDefault("mailCheckInterval", 10)
		viper.SetDefault("roomID", "")
		viper.WriteConfigAs("./cfg.json")
		return true
	}
	return false
}

func loginMatrix() {
	client, err := mautrix.NewClient(viper.GetString("matrixserver"), viper.GetString("matrixuserid"), viper.GetString("matrixaccesstoken"))
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("logged into matrix successfully")
	}
	client.SetDisplayName("jojiiMail")
	matrixClient = client
	go startMatrixSync(client)
}

func startMatrixSync(client *mautrix.Client) {
	fmt.Println(client.UserID)
	filt, err := client.CreateFilter(json.RawMessage("{ \"room\": { \"state\": { \"types\": [ \"m.room.*\" ] } }, \"presence\": { \"types\": [ \"m.presence\" ] }, \"event_fields\": [ \"type\", \"content\", \"sender\", \"room_id\", \"event_id\" ] }"))
	if err != nil {
		log.Panic(err)
		return
	}

	var next = ""
	for {
		syncRes, err := client.SyncRequest(100000, next, filt.FilterID, false, "online")
		if err != nil {
			continue
		}
		next = syncRes.NextBatch

		//autojoin
		invites := syncRes.Rooms.Invite
		if invites != nil {
			for key, val := range invites {
				_ = val
				client.JoinRoom(key, "", nil)
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {

	exit := initCfg()
	if exit {
		return
	}

	er := initDB()
	if er == nil {
		createTable()
	} else {
		return
	}

	loginMatrix()

	host := viper.GetString("emailhost") + ":" + viper.GetString("port")
	fmt.Println(host)
	err := loginMail(host, viper.GetString("username"), viper.GetString("password"))

	if err != nil {
		log.Fatal(err)
	}
	mailCheckTimer()
	defer mailClient.Logout()
}

func mailCheckTimer() {
	for {
		go fetchNewMails()
		interval := viper.GetInt64("mailCheckInterval")
		time.Sleep((time.Duration)(int64(interval)) * time.Second)
	}
}

func fetchNewMails() {
	messages := make(chan *imap.Message, 1)
	section := getMails(messages)

	for msg := range messages {
		mailID := msg.Envelope.Subject + strconv.Itoa(int(msg.InternalDate.Unix()))
		if has, err := dbContainsMail(mailID); !has && err == nil {
			go insertEmail(mailID)
			handleMail(msg, section)
		}
	}
}

func handleMail(mail *imap.Message, section *imap.BodySectionName) {
	if len(viper.GetString("roomID")) == 0 {
		return
	}
	content := getMailContent(mail, section)
	fmt.Println("new Mail:")
	fmt.Println(content.body)
	matrixClient.SendText(viper.GetString("roomID"), "You've got a new Email FROM "+content.from+":")
	matrixClient.SendText(viper.GetString("roomID"), "Subject: "+content.subject)
	matrixClient.SendText(viper.GetString("roomID"), content.body)
}
