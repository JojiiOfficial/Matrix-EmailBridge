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
var db *sql.DB

func initDB() error {
	database, era := sql.Open("sqlite3", "./data.db")
	if era != nil {
		panic(era)
	}
	db = database
	return era
}

//returns true if app should exit
func initCfg() bool {
	viper.SetConfigType("json")
	viper.SetConfigFile("./cfg.json")
	viper.AddConfigPath("./")
	viper.SetConfigName("cfg")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("config not found. creating new one")
		viper.SetDefault("matrixServer", "matrix.org")
		viper.SetDefault("matrixaccesstoken", "hlaksdjhaslkfslkj")
		viper.SetDefault("matrixuserid", "@m:matrix.org")
		viper.SetDefault("defuaultmailCheckInterval", 10)
		viper.WriteConfigAs("./cfg.json")
		return true
	}
	return false
}

func loginMatrix() {
	client, err := mautrix.NewClient(viper.GetString("matrixserver"), viper.GetString("matrixuserid"), viper.GetString("matrixaccesstoken"))
	if err != nil {
		WriteLog(critical, "#02 loggin into matrix account: "+err.Error())
	} else {
		WriteLog(success, "logged into matrix")
		fmt.Println("logged into matrix successfully")
	}
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
		if evt.Sender != client.UserID && evt.Content.Membership == "leave" {
			_, ok := listenerMap[evt.RoomID]
			if ok {
				close(listenerMap[evt.RoomID])
				//delete(listenerMap, evt.RoomID)
			}
			deleteRoomAndEmailByRoomID(evt.RoomID)
		}
	})

	syncer.OnEventType(mautrix.EventMessage, func(evt *mautrix.Event) {
		if evt.Sender == client.UserID {
			return
		}
		message := evt.Content.Body
		roomID := evt.RoomID
		//commands only available in room not bridged to email
		if has, err := hasRoom(roomID); !has && err == nil {
			if message == "!login" {
				client.SendText(roomID, "Okay send me the data of your server(IMAPs) in the given order, splitted by a comma(,)\r\n!setup imap, host:port, username/email, password, mailbox, ignoreSSL\r\n\r\nExample: \r\n!setup imap, host.com:993, mail@host.com, w0rdp4ss, INBOX, false")
			} else if strings.HasPrefix(message, "!setup") {
				data := strings.Trim(strings.ReplaceAll(message, "!setup", ""), " ")
				s := strings.Split(data, ",")
				for i := 0; i < len(s); i++ {
					s[i] = strings.Trim(s[i], " ")
				}
				if len(s) < 4 || len(s) > 6 {
					client.SendText(roomID, "Wrong syntax :/\r\nExample: \r\n!setup imap, host.com:993, mail@host.com, w0rdp4ss, INBOX, false")
				} else {
					accountType := s[0]
					if strings.ToLower(accountType) != "imap" && strings.ToLower(accountType) != "smtp" {
						client.SendText(roomID, "What? you can setup 'imap' and 'smtp', not \""+accountType+"\"")
						return
					}
					host := s[1]
					username := s[2]
					password := s[3]
					ignoreSSlCert := false
					mailbox := "INBOX"
					if len(s) >= 5 {
						mailbox = s[4]
					}
					if len(s) == 6 {
						ignoreSSlCert, err = strconv.ParseBool(s[5])
						if err != nil {
							fmt.Println(err.Error())
							ignoreSSlCert = false
						}
					}
					if accountType == "imap" {
						isInUse, err := isImapAccountAlreadyInUse(username)
						if err != nil {
							client.SendText(roomID, "Something went wrong! Contact the admin. Errorcode: #03")
							WriteLog(critical, "#03 checking isImapAccountAlreadyInUse: "+err.Error())
							return
						}

						if isInUse {
							client.SendText(roomID, "This email is already in Use! You cannot use your email twice!")
							return
						}

						go func() {
							mclient, err := loginMail(host, username, password, ignoreSSlCert)
							if mclient != nil && err == nil {
								id, succes := insertimapAccountount(host, username, password, mailbox, ignoreSSlCert)
								if !succes {
									client.SendText(roomID, "sth went wrong. Contact your admin")
									return
								}
								defaultMailSyncInterval := viper.GetInt("defuaultmailCheckInterval")
								newRoomID := insertNewRoom(roomID, mailbox, int(id), defaultMailSyncInterval)
								if newRoomID == -1 {
									client.SendText(roomID, "An error occured! contact your admin! Errorcode: #19")
									return
								}
								client.SendText(roomID, "Bridge created successfully!\r\nYou should delete the message containing your credentials ;)")
								startMailListener(imapAccountount{host, username, password, roomID, mailbox, ignoreSSlCert, int(newRoomID), defaultMailSyncInterval, true})
								WriteLog(success, "Created new bridge and started maillistener")
							} else {
								client.SendText(roomID, "Error creating bridge! Errorcode: #04\r\nReason: "+err.Error())
								WriteLog(logError, "#04 creating bridge: "+err.Error())
							}
						}()
					} else {
						client.SendText(roomID, "Not implemented yet!")
					}
				}
			}
		} else {
			if err != nil {
				WriteLog(critical, "#05 receiving message: "+err.Error())
				client.SendText(roomID, "sth went wrong. Contact your admin. Errorcode: #05")
			} else {
				switch message {
				case "!ping":
					{
						roomData, err := getRoomInfo(roomID)
						if err != nil {
							WriteLog(logError, "#06 getting roomdata: "+err.Error())
							client.SendText(roomData, "An server-error occured")
							return
						}
						client.SendText(roomID, roomData)
					}
				case "!help":
					{
						helpText := "-------- Help --------\r\n"
						helpText += "!setup imap/smtp, host:port, username(em@ail.com), password, ignoreSSLcert(true/false) - creates a bridge for this room\r\n"
						helpText += "!ping - gets information about the email bridge for this room\r\n"
						helpText += "!help - shows this command help overview\r\n"
						client.SendText(roomID, helpText)
					}
				default:
					{
						if message == "!login" || strings.HasPrefix(message, "!setup") {
							client.SendText(roomID, "This room is already assigned to a emailaddress. Create a new room if you want to bridge a new emailaccount")
						}
					}
				}

			}
		}
	})

	err := client.Sync()
	if err != nil {
		WriteLog(logError, "#07 Syncing: "+err.Error())
		fmt.Println(err)
	}
}

func main() {
	initLogger()

	exit := initCfg()
	if exit {
		return
	}

	er := initDB()
	if er == nil {
		createAllTables()
		WriteLog(success, "create tables")
	} else {
		WriteLog(critical, "#08 creating tables: "+er.Error())
		panic(er)
	}

	loginMatrix()

	startMailSchedeuler()

	for {
		time.Sleep(1 * time.Second)
	}
}

var listenerMap map[string]chan bool

func startMailSchedeuler() {
	listenerMap = make(map[string]chan bool)
	accounts, err := getimapAccounts()
	if err != nil {
		WriteLog(critical, "#09 reading accounts: "+err.Error())
		log.Panic(err)
	}
	for i := 0; i < len(accounts); i++ {
		go startMailListener(accounts[i])
	}
	WriteLog(success, "started "+strconv.Itoa(len(accounts))+" mail listener")
}

func startMailListener(account imapAccountount) {
	quit := make(chan bool)
	mClient, err := loginMail(account.host, account.username, account.password, account.ignoreSSL)
	if err != nil {
		WriteLog(logError, "#10 email account error: "+err.Error())
		matrixClient.SendText(account.roomID, "Error:\r\n"+err.Error()+"\r\n\r\nBecause of this error you have to login to your account again using !setup")
		deleteRoomAndEmailByRoomID(account.roomID)
		return
	}
	listenerMap[account.roomID] = quit
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				fmt.Println("check for " + account.username)
				fetchNewMails(mClient, &account)
				time.Sleep((time.Duration)(account.mailCheckInterval) * time.Second)
			}
		}
	}()
}

func fetchNewMails(mClient *client.Client, account *imapAccountount) {
	messages := make(chan *imap.Message, 1)
	section := getMails(mClient, account.mailbox, messages)

	if section == nil {
		if account.silence {
			account.silence = false
		}
		return
	}

	for msg := range messages {
		mailID := msg.Envelope.Subject + strconv.Itoa(int(msg.InternalDate.Unix()))
		if has, err := dbContainsMail(mailID, account.roomPKID); !has && err == nil {
			go insertEmail(mailID, account.roomPKID)
			if !account.silence {
				handleMail(msg, section, *account)
			}
		} else if err != nil {
			WriteLog(logError, "#11 dbContains mail: "+err.Error())
			fmt.Println(err.Error())
		}
	}
	if account.silence {
		account.silence = false
	}
}

func handleMail(mail *imap.Message, section *imap.BodySectionName, account imapAccountount) {
	content := getMailContent(mail, section)
	fmt.Println(content.body)
	matrixClient.SendText(account.roomID, "You've got a new Email FROM "+content.from)
	matrixClient.SendText(account.roomID, "Subject: "+content.subject)
	matrixClient.SendText(account.roomID, content.body)
}
