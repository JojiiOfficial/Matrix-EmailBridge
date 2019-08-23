package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
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
		viper.SetDefault("defuaultmailCheckInterval", 10)
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

	store := NewFileStore("store.json")
	store.Load()
	client.Store = store

	go startMatrixSync(client)
}

func startMatrixSync(client *mautrix.Client) {
	fmt.Println(client.UserID)

	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(mautrix.StateJoinRules, func(evt *mautrix.Event) {
		client.JoinRoom(evt.RoomID, "", nil)
		client.SendText(evt.RoomID, "Hey you have invited me to a new room. Enter !login to bridge this room to a Mail account")
	})
	syncer.OnEventType(mautrix.StateMember, func(evt *mautrix.Event) {
		fmt.Printf("<%[1]s> %[4]s (%[2]s/%[3]s)\n", evt.Sender, evt.Type.String(), evt.ID, evt.Content.Body)
		fmt.Println(evt.Content.Membership)
		if evt.Sender != client.UserID && evt.Content.Membership == "leave" {
			deleteRoomAndEmailByRoomID(evt.RoomID)
		}
	})

	syncer.OnEventType(mautrix.EventMessage, func(evt *mautrix.Event) {
		if evt.Sender == client.UserID {
			return
		}
		fmt.Printf("<%[1]s> %[4]s (%[2]s/%[3]s)\n", evt.Sender, evt.Type.String(), evt.ID, evt.Content.Body)
		message := evt.Content.Body
		roomID := evt.RoomID
		//commands only available in room not bridged to email
		if has, err := hasRoom(roomID); !has && err == nil {
			if message == "!login" {
				client.SendText(roomID, "Okay send me the data of your server(IMAPs) in the given order, splitted by a comma(,)\r\n!setup host:port, username/email, password\r\n\r\nExample: \r\n!setup host.com:993, mail@host.com, w0rdp4ss")
			} else if strings.HasPrefix(message, "!setup") {
				data := strings.Trim(strings.ReplaceAll(message, "!setup", ""), " ")
				s := strings.Split(data, ",")
				for i := 0; i < len(s); i++ {
					s[i] = strings.Trim(s[i], " ")
				}
				if len(s) < 3 || len(s) > 4 {
					client.SendText(roomID, "Wrong syntax :/\r\nExample: \r\n!setup host.com:993, mail@host.com, w0rdp4ss")
				} else {
					host := s[0]
					username := s[1]
					password := s[2]
					ignoreSSlCert := false
					if len(s) == 4 {
						ignoreSSlCert, err = strconv.ParseBool(s[3])
						if err != nil {
							ignoreSSlCert = false
						}
					}
					id, success := insertEmailAccount(host, username, password, ignoreSSlCert)
					if !success {
						client.SendText(roomID, "sth went wrong. Contact your admin")
						return
					}
					insertNewRoom(roomID, int(id), viper.GetInt("defuaultmailCheckInterval"))
					client.SendText(roomID, "Bridge created successfully!")
				}
			}
		} else {
			if err != nil {
				client.SendText(roomID, "sth went wrong. Contact your admin")
			} else {
				if message == "!login" || strings.HasPrefix(message, "!setup") {
					client.SendText(roomID, "This room is already assigned to a emailaddress. Create a new room if you want to bridge a new emailaccount")
				} else {
					//commands always available

				}
			}
		}
	})

	err := client.Sync()
	if err != nil {
		fmt.Println(err)
	}
}

func main() {

	exit := initCfg()
	if exit {
		return
	}

	er := initDB()
	if er == nil {
		createAllTables()
	} else {
		panic(er)
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
		interval := viper.GetInt64("defuaultmailCheckInterval")
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
	matrixClient.SendText(viper.GetString("roomID"), "You've got a new Email FROM defuaultmailCheckInterval+content.from+")
	matrixClient.SendText(viper.GetString("roomID"), "Subject: "+content.subject)
	matrixClient.SendText(viper.GetString("roomID"), content.body)
}
