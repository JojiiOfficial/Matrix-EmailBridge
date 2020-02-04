package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"html"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-sasl"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"github.com/tulir/mautrix-go"
	"gopkg.in/gomail.v2"
)

var matrixClient *mautrix.Client
var db *sql.DB

const tempDir = "./temp/"
const version = 6

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

		rr := saveVersion(version)
		if rr != nil {
			panic(rr)
		}

		viper.SetDefault("matrixServer", "matrix.org")
		viper.SetDefault("matrixaccesstoken", "hlaksdjhaslkfslkj")
		viper.SetDefault("matrixuserid", "@m:matrix.org")
		viper.SetDefault("defaultmailCheckInterval", 30)
		viper.SetDefault("markdownEnabledByDefault", true)
		viper.SetDefault("htmlDefault", false)
		viper.SetDefault("allowed_servers", [1]string{"YourMatrixServerDomain.com"})
		viper.WriteConfigAs("./cfg.json")
		return true
	}

	ae := viper.GetInt("defaultmailCheckInterval")
	if ae == 0 {
		viper.SetDefault("defaultmailCheckInterval", 1)
		viper.WriteConfigAs("./cfg.json")
	}

	allowedHosts := viper.GetStringSlice("allowed_servers")
	if len(allowedHosts) == 0 {
		allowedHosts = make([]string, 1)
		allowedHosts[0] = "YourMatrixServerDomain.com"
		viper.SetDefault("allowed_servers", allowedHosts)
		viper.WriteConfigAs("./cfg.json")
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

func getHostFromMatrixID(matrixID string) (host string, err int) {
	if strings.Contains(matrixID, ":") {
		splt := strings.Split(matrixID, ":")
		if len(splt) == 2 {
			return splt[1], -1
		}
		return "", 1
	}
	return "", 0
}

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func logOut(client *mautrix.Client, roomID string, leave bool) error {
	stopMailChecker(roomID)
	deleteRoomAndEmailByRoomID(roomID)
	if leave {
		_, err := client.LeaveRoom(roomID)
		if err != nil {
			WriteLog(critical, "#65 bot can't leave room: "+err.Error())
			return err
		}
	}
	return nil
}

func startMatrixSync(client *mautrix.Client) {
	fmt.Println(client.UserID)

	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(mautrix.StateJoinRules, func(evt *mautrix.Event) {
		host, err := getHostFromMatrixID(evt.Sender)
		if err == -1 {
			listcontains := contains(viper.GetStringSlice("allowed_servers"), host)
			if listcontains {
				client.JoinRoom(evt.RoomID, "", nil)
				client.SendText(evt.RoomID, "Hey you have invited me to a new room. Enter !login to bridge this room to a Mail account")
			} else {
				client.LeaveRoom(evt.RoomID)
				WriteLog(info, "Got invalid invite from "+evt.Sender+" reason: senders server not whitelisted! Adjust your config if you want to allow this host using me")
				return
			}
		} else {
			WriteLog(critical, "")
		}
	})

	syncer.OnEventType(mautrix.StateMember, func(evt *mautrix.Event) {
		if evt.Sender != client.UserID && evt.Content.Membership == "leave" {
			logOut(client, evt.RoomID, true)
		}
	})

	syncer.OnEventType(mautrix.EventMessage, func(evt *mautrix.Event) {
		if evt.Sender == client.UserID {
			return
		}
		message := evt.Content.Body
		roomID := evt.RoomID

		if is, err := isUserWritingEmail(roomID); is && err == nil {
			writeTemp, err := getWritingTemp(roomID)
			if err != nil {
				WriteLog(critical, "#43 getWritingTemp: "+err.Error())
				client.SendText(roomID, "An server-error occured Errorcode: #43")
				deleteWritingTemp(roomID)
				return
			}
			if len(strings.Trim(writeTemp.subject, " ")) == 0 {
				if evt.Content.MsgType != mautrix.MsgText {
					client.SendText(roomID, "You have to send a text for subject!")
					return
				}
				err = saveWritingtemp(roomID, "subject", message)
				if err != nil {
					WriteLog(critical, "#44 saveWritingtemp: "+err.Error())
					client.SendText(roomID, "An server-error occured Errorcode: #44")
					deleteWritingTemp(roomID)
					return
				}
				client.SendText(roomID, "Now send me the content of the email. One message is one line. If you want to send or cancle enter !send or !cancel")
			} else {
				if message == "!send" {
					account, err := getSMTPAccount(roomID)
					if err != nil {
						WriteLog(critical, "#52 saveWritingtemp: "+err.Error())
						client.SendText(roomID, "An server-error occured Errorcode: #52")
						deleteWritingTemp(roomID)
						return
					}

					m := gomail.NewMessage()
					m.SetHeader("From", account.username)

					if strings.Contains(writeTemp.receiver, ",") {
						recEmails := strings.Split(writeTemp.receiver, ",")
						m.SetHeader("To", recEmails...)
					} else {
						m.SetHeader("To", writeTemp.receiver)
					}

					m.SetHeader("Subject", writeTemp.subject)

					if writeTemp.markdown {
						toSendText := string(markdown.ToHTML([]byte(writeTemp.body), nil, nil))
						toSendText = strings.ReplaceAll(toSendText, "\r\n<h", "<h")
						toSendText = strings.ReplaceAll(toSendText, "\n\n<h", "<h")
						toSendText = strings.ReplaceAll(toSendText, ">\n\n", ">")
						toSendText = strings.ReplaceAll(toSendText, "\r\n", "<br>")
						m.SetBody("text/html", toSendText)

						plainbody := writeTemp.body
						plainbody = strings.ReplaceAll(plainbody, "<br>", "\r\n")
						m.AddAlternative("text/plain", plainbody)
					} else {
						m.SetBody("text/plain", writeTemp.body)
					}

					attachments, err := getAttachments(writeTemp.pkID)
					if err == nil {
						for _, i := range attachments {
							client.SendText(roomID, "Attaching file: "+i)
							m.Attach(tempDir + i)
						}
					} else {
						client.SendText(roomID, "coulnd't attach files: "+err.Error())
					}

					d := gomail.NewDialer(account.host, account.port, account.username, account.password)
					if account.ignoreSSL {
						d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
					}
					client.SendText(roomID, "Sending...")
					if err := d.DialAndSend(m); err != nil {
						WriteLog(logError, "#46 DialAndSend: "+err.Error())
						client.SendText(roomID, "An server-error occured Errorcode: #53\r\n"+err.Error())
						removeSMTPAccount(roomID)
						client.SendText(roomID, "To fix this errer you have to run !setup smtp .... again")
						deleteWritingTemp(roomID)
						return
					}
					client.SendText(roomID, "Message sent successfully")
					deleteWritingTemp(roomID)
				} else if message == "!cancel" {
					client.SendText(roomID, "Mail canceled")
					deleteWritingTemp(roomID)
					return
				} else if strings.HasPrefix(message, "!rm") && len(strings.Split(message, " ")) > 0 {
					splitted := strings.Split(message, " ")[1:]
					var fileName string
					for _, a := range splitted {
						fileName += a + " "
					}
					fileName = strings.TrimRight(fileName, " ")
					fileName = strings.TrimLeft(fileName, " ")
					fmt.Println(fileName)
					err := deleteAttachment(fileName, writeTemp.pkID)
					if err != nil {
						client.SendText(roomID, "Couldn't delete attachment: "+err.Error())
						return
					}
					_ = os.Remove(tempDir + fileName)
					client.SendText(roomID, "Attachment deleted!")

				} else {
					if evt.Content.MsgType == mautrix.MsgText {
						if len(strings.ReplaceAll(writeTemp.body, " ", "")) == 0 {
							err = saveWritingtemp(roomID, "body", message+"\r\n")
						} else {
							err = saveWritingtemp(roomID, "body", writeTemp.body+message+"\r\n")
						}
						if err != nil {
							WriteLog(critical, "#54 saveWritingtemp: "+err.Error())
							client.SendText(roomID, "An server-error occured Errorcode: #54")
							deleteWritingTemp(roomID)
							return
						}
					} else if evt.Content.MsgType == mautrix.MsgFile || evt.Content.MsgType == mautrix.MsgImage {
						if strings.HasPrefix(evt.Content.URL, "mxc://") {
							reader, err := client.Download(evt.Content.URL)
							if err != nil {
								client.SendText(roomID, "Couldn't download File: "+err.Error())
							} else {
								filename := strconv.Itoa(int(time.Now().Unix())) + "_" + evt.Content.Body
								err := streamToTempFile(reader, filename)
								if err != nil {
									client.SendText(roomID, "Couldn't download file: "+err.Error())
								} else {
									addEmailAttachment(writeTemp.pkID, filename)
									client.SendText(roomID, "File "+filename+" attached!")
								}
							}
						}
					}

				}
			}
		} else if err != nil {
			WriteLog(critical, "#41 deleteWritingTemp: "+err.Error())
			client.SendText(roomID, "An server-error occured Errorcode: #41")
			return
		} else {
			//commands only available in room not bridged to email
			if message == "!login" {
				client.SendText(roomID, "Okay send me the data of your server(at first IMAPs) in the given order, splitted by a comma(,)\r\n!setup imap, host:port, username/email, password, mailbox, ignoreSSL\r\n!setup smtp, host, port, email, password, ignoreSSL\r\n\r\nExample: \r\n!setup imap, host.com:993, mail@host.com, w0rdp4ss, INBOX, false\r\nor\r\n!setup smtp, host.com:587, mail@host.com, w0rdp4ss, false")
			} else if strings.HasPrefix(message, "!setup") {
				data := strings.Trim(strings.ReplaceAll(message, "!setup", ""), " ")
				s := strings.Split(data, ",")
				if len(s) < 4 || len(s) > 6 {
					client.SendText(roomID, "Wrong syntax :/\r\nExample: \r\n!setup imap, host.com:993, mail@host.com, w0rdp4ss, INBOX, false\r\nor\r\n"+
						"!setup smtp, host.com:587, mail@host.com, w0rdp4ss, false")
				} else {
					accountType := s[0]
					if strings.ToLower(accountType) != "imap" && strings.ToLower(accountType) != "smtp" {
						client.SendText(roomID, "What? you can setup 'imap' and 'smtp', not \""+accountType+"\"")
						return
					}
					host := strings.ReplaceAll(s[1], " ", "")
					username := strings.ReplaceAll(s[2], " ", "")
					password := strings.ReplaceAll(s[3], " ", "")
					ignoreSSlCert := false
					mailbox := "INBOX"
					if len(s) >= 5 {
						mailbox = strings.ReplaceAll(s[4], " ", "")
					}
					var err error
					defaultMailSyncInterval := viper.GetInt("defaultmailCheckInterval")
					imapAccID, smtpAccID, erro := getRoomAccounts(roomID)
					if erro != nil {
						client.SendText(roomID, "Something went wrong! Contact the admin. Errorcode: #37")
						WriteLog(critical, "#37 checking getRoomAccounts: "+erro.Error())
						return
					}
					if accountType == "imap" {
						if len(s) == 6 {
							ignoreSSlCert, err = strconv.ParseBool(strings.ReplaceAll(s[5], " ", ""))
							if err != nil {
								fmt.Println(err.Error())
								ignoreSSlCert = false
							}
						}
						if imapAccID != -1 {
							client.SendText(roomID, "IMAP account already existing. Create a new room if you want to use a different account!")
							return
						}
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
							if !strings.Contains(host, ":") {
								host += ":993"
							}

							mclient, err := loginMail(host, username, password, ignoreSSlCert)
							if mclient != nil && err == nil {
								has, er := hasRoom(roomID)
								if er != nil {
									client.SendText(roomID, "An error occured! contact your admin! Errorcode: #25")
									WriteLog(critical, "checking imapAcc #25: "+er.Error())
									return
								}
								var newRoomID int64
								if !has {
									newRoomID = insertNewRoom(roomID, defaultMailSyncInterval)
									if newRoomID == -1 {
										client.SendText(roomID, "An error occured! contact your admin! Errorcode: #26")
										WriteLog(critical, "checking insertNewRoom #26")
										return
									}
								} else {
									id, err := getRoomPKID(evt.RoomID)
									if err != nil {
										WriteLog(critical, "checking getRoomPKID #27: "+err.Error())
										client.SendText(roomID, "An error occured! contact your admin! Errorcode: #27")
										return
									}
									newRoomID = int64(id)
								}
								imapID, succes := insertimapAccountount(host, username, password, mailbox, ignoreSSlCert)
								if !succes {
									client.SendText(roomID, "sth went wrong. Contact your admin")
									return
								}
								err = saveImapAcc(roomID, int(imapID))
								if err != nil {
									WriteLog(critical, "saveImapAcc #35 : "+err.Error())
									client.SendText(roomID, "sth went wrong. Contact you admin! Errorcode: #35")
									return
								}
								client.SendText(roomID, "Bridge created successfully!\r\nYou should delete the message containing your credentials ;)\r\nIMAP:\r\n"+
									"host: "+host+"\r\n"+
									"username: "+username+"\r\n"+
									"mailbox: "+mailbox+"\r\n"+
									"ignoreSSL: "+strconv.FormatBool(ignoreSSlCert))

								startMailListener(imapAccountount{host, username, password, roomID, mailbox, ignoreSSlCert, int(newRoomID), defaultMailSyncInterval, true})
								WriteLog(success, "Created new bridge and started maillistener\r\n")
							} else {
								client.SendText(roomID, "Error creating bridge! Errorcode: #04\r\nReason: "+err.Error())
								WriteLog(logError, "#04 creating bridge: "+err.Error())
							}
						}()
					} else if accountType == "smtp" {
						if smtpAccID != -1 {
							client.SendText(roomID, "SMTP account already existing. Create a new room if you want to use a different account!")
							return
						}
						isInUse, err := isSMTPAccountAlreadyInUse(username)
						if err != nil {
							client.SendText(roomID, "Something went wrong! Contact the admin. Errorcode: #24")
							WriteLog(critical, "#24 checking isSMTPAccountAlreadyInUse: "+err.Error())
							return
						}
						if isInUse {
							client.SendText(roomID, "This smtp-username is already in Use! You cannot use your email twice!")
							return
						}

						go func() {
							if len(s) == 5 {
								ignoreSSlCert, err = strconv.ParseBool(strings.ReplaceAll(s[4], " ", ""))
								if err != nil {
									fmt.Println(err.Error())
									ignoreSSlCert = false
								}
							}
							has, er := hasRoom(roomID)
							if er != nil {
								client.SendText(roomID, "An error occured! contact your admin! Errorcode: #28")
								WriteLog(critical, "checking imapAcc #28: "+er.Error())
								return
							}
							var newRoomID int64
							if !has {
								newRoomID = insertNewRoom(roomID, defaultMailSyncInterval)
								if newRoomID == -1 {
									client.SendText(roomID, "An error occured! contact your admin! Errorcode: #29")
									WriteLog(critical, "checking insertNewRoom #29: ")
									return
								}
							} else {
								id, err := getRoomPKID(evt.RoomID)
								if err != nil {
									WriteLog(critical, "checking getRoomPKID #30: "+err.Error())
									client.SendText(roomID, "An error occured! contact your admin! Errorcode: #30")
									return
								}
								newRoomID = int64(id)
							}
							port := 587
							if !strings.Contains(host, ":") {
								client.SendText(roomID, "No port specified! Using 587")
							} else {
								hostsplit := strings.Split(host, ":")
								host = hostsplit[0]
								port, err = strconv.Atoi(strings.Trim(hostsplit[1], " "))
								if err != nil {
									client.SendText(roomID, "The port must be a number!")
									return
								}
							}
							smtpID, err := insertSMTPAccountount(host, port, username, password, ignoreSSlCert)
							if err != nil {
								client.SendText(roomID, "sth went wrong. Contact your admin")
								return
							}
							err = saveSMTPAcc(roomID, int(smtpID))
							if err != nil {
								WriteLog(critical, "saveSMTPAcc #36 : "+err.Error())
								client.SendText(roomID, "sth went wrong. Contact you admin! Errorcode: #34")
								return
							}

							client.SendText(roomID, "SMTP data saved.\r\nSMTP:\r\n"+
								"host: "+host+"\r\n"+
								"port: "+strconv.Itoa(port)+"\r\n"+
								"username: "+username+"\r\n"+
								"ignoreSSL: "+strconv.FormatBool(ignoreSSlCert))
						}()
					} else {
						client.SendText(roomID, "Not implemented yet!")
					}
				}
			} else if message == "!help" {
				helpText := "-------- Help --------\r\n"
				helpText += "!setup imap/smtp, host:port, username(em@ail.com), password, <mailbox (only for imap)>, ignoreSSLcert(true/false) - creates a bridge for this room\r\n"
				helpText += "!ping - gets information about the email bridge for this room\r\n"
				helpText += "!help - shows this command help overview\r\n"
				helpText += "!write (receiver(s) email(s) splitted by space!) <markdown default:true>- sends an email to a given address\r\n"
				helpText += "!mailboxes - shows a list with all mailboxes available on your IMAP server\r\n"
				helpText += "!setmailbox (mailbox) - changes the mailbox for the room\r\n"
				helpText += "!mailbox - shows the currently selected mailbox\r\n"
				helpText += "!sethtml (on/off or true/false) - sets HTML-rendering for messages on/off\r\n"
				helpText += "!logout remove email bridge from current room\r\n"
				helpText += "!leave unbridge the current room and kick the bot\r\n"
				helpText += "\r\n---- Email writing commands ----\r\n"
				helpText += "!send - sends the email\r\n"
				helpText += "!rm <file> - removes given attachment from email\r\n"
				client.SendText(roomID, helpText)
			} else if message == "!ping" {
				if has, err := hasRoom(roomID); has && err == nil {
					roomData, err := getRoomInfo(roomID)
					if err != nil {
						WriteLog(logError, "#006 getRoomInfo: "+err.Error())
						client.SendText(roomID, "An server-error occured")
						return
					}

					client.SendText(roomID, roomData)
				} else {
					if err != nil {
						WriteLog(logError, "#06 hasRoom: "+err.Error())
						client.SendText(roomID, "An server-error occured")
					} else {
						client.SendText(roomID, "You have to login to use this command!")
					}
				}
			} else if strings.HasPrefix(message, "!write") {
				if has, err := hasRoom(roomID); has && err == nil {
					_, smtpAccID, erro := getRoomAccounts(roomID)
					if erro != nil {
						WriteLog(critical, "#38 getRoomAccounts: "+erro.Error())
						client.SendText(roomID, "An server-error occured Errorcode: #38")
						return
					}
					if smtpAccID == -1 {
						client.SendText(roomID, "You have to setup an smtp account. Type !help or !login for more information")
						return
					}
					s := strings.Split(message, " ")
					if len(s) > 1 {
						receiver := strings.Trim(s[1], " ")
						if len(s) > 2 {
							receiverString := ""
							for i := 1; i < len(s); i++ {
								recEmail := strings.Trim(s[i], " ")
								if len(recEmail) == 0 || !strings.Contains(recEmail, "@") || !strings.Contains(recEmail, ".") || strings.Contains(receiverString, recEmail) {
									continue
								}
								add := ","
								if strings.HasSuffix(recEmail, ",") {
									add = ""
								}
								receiverString += strings.Trim(s[i], " ") + add
							}
							receiver = receiverString[:len(receiverString)-1]
						}

						if strings.Contains(receiver, "@") && strings.Contains(receiver, ".") && len(receiver) > 5 {
							hasTemp, err := isUserWritingEmail(roomID)
							if err != nil {
								WriteLog(critical, "#39 isUserWritingEmail: "+err.Error())
								client.SendText(roomID, "An server-error occured Errorcode: #39")
								return
							}
							if hasTemp {
								er := deleteWritingTemp(roomID)
								if er != nil {
									WriteLog(critical, "#40 deleteWritingTemp: "+er.Error())
									client.SendText(roomID, "An server-error occured Errorcode: #40")
									return
								}
							}

							mrkdwn := 0
							if viper.GetBool("markdownEnabledByDefault") {
								mrkdwn = 1
							}
							if len(s) == 3 {
								mdwn, berr := strconv.ParseBool(s[2])
								if berr == nil {
									if mdwn {
										mrkdwn = 1
									} else {
										mrkdwn = 0
									}
								}
							}

							err = newWritingTemp(roomID, receiver)
							saveWritingtemp(roomID, "markdown", strconv.Itoa(mrkdwn))
							if err != nil {
								WriteLog(critical, "#42 newWritingTemp: "+err.Error())
								client.SendText(roomID, "An server-error occured Errorcode: #42")
								return
							}
							client.SendText(roomID, "Now send me the subject of your email")
						} else {
							client.SendText(roomID, "this is an email: max@google.de\r\nthis is no email: "+receiver)
						}
					} else {
						client.SendText(roomID, "Usage: !write <emailaddress>")
					}
				} else {
					client.SendText(roomID, "You have to login to use this command!")
				}
			} else if message == "!mailboxes" {
				imapAccID, _, erro := getRoomAccounts(roomID)
				if erro != nil {
					WriteLog(critical, "#48 getRoomAccounts: "+erro.Error())
					client.SendText(roomID, "An server-error occured Errorcode: #48")
					return
				}
				if imapAccID != -1 {
					mailboxes, err := getMailboxes(clients[roomID])
					if err != nil {
						WriteLog(critical, "#47 getMailboxes: "+err.Error())
						client.SendText(roomID, "An server-error occured Errorcode: #47")
						return
					}
					client.SendText(roomID, "Your mailboxes:\r\n"+mailboxes+"\r\nUse !setmailbox <mailbox> to change your mailbox")
				} else {
					client.SendText(roomID, "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
				}
			} else if strings.HasPrefix(message, "!setmailbox") {
				imapAccID, _, erro := getRoomAccounts(roomID)
				if erro != nil {
					WriteLog(critical, "#48 getRoomAccounts: "+erro.Error())
					client.SendText(roomID, "An server-error occured Errorcode: #48")
					return
				}
				if imapAccID != -1 {
					d := strings.Split(message, " ")
					if len(d) == 2 {
						mailbox := d[1]
						saveMailbox(roomID, mailbox)
						deleteMails(roomID)
						stopMailChecker(roomID)
						imapAccount, err := getIMAPAccount(roomID)
						if err != nil {
							WriteLog(critical, "#49 getIMAPAccount: "+err.Error())
							client.SendText(roomID, "An server-error occured Errorcode: #49")
							return
						}
						imapAccount.silence = true
						go startMailListener(*imapAccount)
						client.SendText(roomID, "Mailbox updated")
					} else {
						client.SendText(roomID, "Usage: !setmailbox <new mailbox>")
					}
				} else {
					client.SendText(roomID, "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
				}
			} else if message == "!mailbox" {
				imapAccID, _, erro := getRoomAccounts(roomID)
				if erro != nil {
					WriteLog(critical, "#50 getRoomAccounts: "+erro.Error())
					client.SendText(roomID, "An server-error occured Errorcode: #50")
					return
				}
				if imapAccID != -1 {
					mailbox, err := getMailbox(roomID)
					if err != nil {
						WriteLog(critical, "#51 getMailbox: "+err.Error())
						client.SendText(roomID, "An server-error occured Errorcode: #51")
						return
					}
					client.SendText(roomID, "The current mailbox for this room is: "+mailbox)
				} else {
					client.SendText(roomID, "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
				}
			} else if strings.HasPrefix(message, "!sethtml") {
				imapAccID, _, erro := getRoomAccounts(roomID)
				if erro != nil {
					WriteLog(critical, "#50 getRoomAccounts: "+erro.Error())
					client.SendText(roomID, "An server-error occured Errorcode: #50")
					return
				}
				if imapAccID != -1 {
					d := strings.Split(message, " ")
					if len(d) == 2 {
						newMode := strings.ToLower(d[1])
						newModeB := false
						if newMode == "true" || newMode == "on" {
							newModeB = true
						} else if newMode != "false" && newMode != "off" {
							client.SendText(roomID, "What?\r\non/off or true/false")
							return
						}
						err := setHTMLenabled(roomID, newModeB)
						if err != nil {
							WriteLog(critical, "#56 getMailbox: "+err.Error())
							client.SendText(roomID, "An server-error occured Errorcode: #56")
							return
						}
						client.SendText(roomID, "Successfully set HTML-rendering to "+newMode)
					} else {
						client.SendText(roomID, "Usage: !sethtml (on/of) or (true/false)")
					}
				} else {
					client.SendText(roomID, "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
				}
			} else if message == "!logout" {
				err := logOut(client, roomID, false)
				if err != nil {
					client.SendText(roomID, "Error logging out: "+err.Error())
				} else {
					client.SendText(roomID, "Successfully logged out")
				}
			} else if message == "!leave" {
				err := logOut(client, roomID, true)
				if err != nil {
					client.SendText(roomID, "Error leaving: "+err.Error())
				} else {
					client.SendText(roomID, "Successfully unbridged")
				}
			} else if strings.HasPrefix(message, "!") {
				client.SendText(roomID, "command not found!")
			}
		}
	})

	err := client.Sync()
	if err != nil {
		WriteLog(logError, "#07 Syncing: "+err.Error())
		fmt.Println(err)
	}
}

func deleteTempFile(name string) {
	os.Remove(tempDir + name)
}

func streamToTempFile(stream io.ReadCloser, file string) error {
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		os.Mkdir(tempDir, os.ModePerm)
	}
	fo, err := os.Create(tempDir + file)
	if err != nil {
		return err
	}

	defer func() {
		if err := fo.Close(); err != nil {

		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := fo.Write(buf[:n]); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	initLogger()

	er := initDB()
	if er == nil {
		createAllTables()
		exit := initCfg()
		if exit {
			return
		}
		WriteLog(success, "create tables")
		handleDBVersion()
	} else {
		WriteLog(critical, "#08 creating tables: "+er.Error())
		panic(er)
	}

	deleteAllWritingTemps()

	loginMatrix()

	startMailSchedeuler()

	for {
		time.Sleep(1 * time.Second)
	}
}

func stopMailChecker(roomID string) {
	_, ok := listenerMap[roomID]
	if ok {
		close(listenerMap[roomID])
		//delete(listenerMap, evt.RoomID)
	}
}

var listenerMap map[string]chan bool
var clients map[string]*client.Client
var imapErrors map[string]*imapError
var checksPerAccount map[string]int

const maxRoomChecks = 15

const maxErrUntilReconnect = 10

type imapError struct {
	retryCount, loginErrCount int
}

func getChecksForAccount(roomID string) int {
	checks, ok := checksPerAccount[roomID]
	if ok {
		return checks
	}
	checksPerAccount[roomID] = 0
	return 0
}

func hasError(roomID string) (has bool, count int) {
	_, ok := imapErrors[roomID]
	if ok {
		return true, imapErrors[roomID].retryCount
	}
	return false, -1
}

func startMailSchedeuler() {
	listenerMap = make(map[string]chan bool)
	clients = make(map[string]*client.Client)
	imapErrors = make(map[string]*imapError)
	checksPerAccount = make(map[string]int)

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
	connectSuccess := false
	var mClient *client.Client
	var err error
	for !connectSuccess {
		mClient, err = loginMail(account.host, account.username, account.password, account.ignoreSSL)
		if err == nil {
			connectSuccess = true
			continue
		} else {
			WriteLog(info, "couldn't connect to imap server try again n a some minutes: "+err.Error())
			time.Sleep(1 * time.Minute)
		}
	}

	listenerMap[account.roomID] = quit
	clients[account.roomID] = mClient
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				if getChecksForAccount(account.roomID) >= maxRoomChecks {
					reconnect(account)
					return
				}
				fetchNewMails(mClient, &account)
				checksPerAccount[account.roomID]++
				time.Sleep((time.Duration)(account.mailCheckInterval) * time.Second)
			}
		}
	}()
}

func reconnect(account imapAccountount) {
	WriteLog(info, "reconnecting account "+account.username)
	checksPerAccount[account.roomID] = 0
	stopMailChecker(account.roomID)
	nacc := account
	go startMailListener(nacc)
}

func fetchNewMails(mClient *client.Client, account *imapAccountount) {
	messages := make(chan *imap.Message, 1)
	section, errCode := getMails(mClient, account.mailbox, messages)

	if section == nil {
		if errCode == 0 {
			haserr, errCount := hasError(account.roomID)
			if haserr {
				if imapErrors[account.roomID].loginErrCount > 15 {
					WriteLog(logError, "Youve got too much errors for the emailaccount: "+account.username)
				}
				if errCount < maxErrUntilReconnect {
					imapErrors[account.roomID].retryCount++
				} else {
					imapErrors[account.roomID].retryCount = 0
					imapErrors[account.roomID].loginErrCount++
					reconnect(*account)
					return
				}
			}
		}
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
	content := getMailContent(mail, section, account.roomID)
	if content == nil {
		return
	}
	fmt.Println(content.body)
	from := html.EscapeString(content.from)
	fmt.Println("attachments: " + content.attachment)
	headerContent := &mautrix.Content{
		Format:        mautrix.FormatHTML,
		Body:          "\r\n────────────────────────────────────\r\n## You've got a new Email from " + from + "\r\n" + "Subject: " + content.subject + "\r\n" + "────────────────────────────────────",
		FormattedBody: "<br>────────────────────────────────────<br><b> You've got a new Email</b> from <b>" + from + "</b><br>" + "Subject: " + content.subject + "<br>" + "────────────────────────────────────",
		MsgType:       mautrix.MsgText,
	}
	matrixClient.SendMessageEvent(account.roomID, mautrix.EventMessage, &headerContent)

	if content.htmlFormat {
		bodyContent := &mautrix.Content{
			Format:        mautrix.FormatHTML,
			Body:          content.body,
			FormattedBody: string(markdown.ToHTML([]byte(content.body), nil, nil)),
			MsgType:       mautrix.MsgText,
		}
		matrixClient.SendMessageEvent(account.roomID, mautrix.EventMessage, &bodyContent)
	} else {
		matrixClient.SendText(account.roomID, content.body)
	}
}
