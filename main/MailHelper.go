package main

import (
	"crypto/tls"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"maunium.net/go/mautrix/id"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	strip "github.com/grokify/html-strip-tags-go"
	"maunium.net/go/mautrix"
)

func loginMail(host, username, password string, ignoreSSL bool) (*client.Client, error) {
	ailClient, err := client.DialTLS(host, &tls.Config{InsecureSkipVerify: ignoreSSL})

	if err != nil {
		return nil, err
	}

	if err := ailClient.Login(username, password); err != nil {
		return nil, err
	}

	return ailClient, nil
}

func getMails(mClient *client.Client, mBox string, messages chan *imap.Message) (*imap.BodySectionName, int) {
	mbox, err := mClient.Select(mBox, false)
	if err != nil {
		WriteLog(logError, "#12 couldnt get INBOX "+err.Error())
		return nil, 0
	}

	if mbox == nil {
		WriteLog(logError, "#23 getMails mbox is nli")
		return nil, 0
	}

	if mbox.Messages == 0 {
		WriteLog(logError, "#13 getMails no messages in inbox ")
		return nil, 1
	}

	seqSet := new(imap.SeqSet)
	maxMessages := uint32(5)
	if mbox.Messages < maxMessages {
		maxMessages = mbox.Messages
	}
	for i := uint32(0); i < maxMessages; i++ {
		seqSet.AddNum(mbox.Messages - i)
	}

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, section.FetchItem()}
	go func() {
		if err := mClient.Fetch(seqSet, items, messages); err != nil {
			WriteLog(critical, "#14 couldnt fetch messages: "+err.Error())
		}
	}()
	return section, -1
}

type email struct {
	body, from, to, subject, attachment string
	sendermails                         []string
	date                                time.Time
	htmlFormat                          bool
}

func getMailboxes(emailClient *client.Client) (string, error) {
	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 20)
	done := make(chan error, 1)
	go func() {
		done <- emailClient.List("", "*", mailboxes)
	}()

	mboxes := ""
	for m := range mailboxes {
		mboxes += "-> " + m.Name + "\r\n"
	}

	if err := <-done; err != nil {
		return "", err
	}
	return mboxes, nil
}

func getMailContent(msg *imap.Message, section *imap.BodySectionName, roomID string) *email {
	if msg == nil {
		fmt.Println("msg is nil")
		WriteLog(logError, "#15 getMailContent msg is nil")
		return nil
	}

	r := msg.GetBody(section)
	if r == nil {
		fmt.Println("reader is nli")
		WriteLog(logError, "#16 getMailContent r (reader) is nil")
		return nil
	}

	jmail := email{}
	mr, err := mail.CreateReader(r)
	if err != nil {
		fmt.Println(err.Error())
		WriteLog(logError, "#17 getMailContent create reader err: "+err.Error())
		return nil
	}

	header := mr.Header
	if date, err := header.Date(); err == nil {
		log.Println("Date:", date)
		jmail.date = date
	}
	if from, err := header.AddressList("From"); err == nil {
		list := make([]string, len(from))
		for i, sender := range from {
			if len(sender.Name) > 0 {
				list[i] = sender.Name + "<" + sender.Address + ">"
			} else {
				list[i] = sender.Address
			}
			jmail.sendermails = append(jmail.sendermails, sender.Address)
		}
		jmail.from = strings.Join(list, ",")
	}
	if to, err := header.AddressList("To"); err == nil {
		list := make([]string, len(to))
		for i, receiver := range to {
			if len(receiver.Name) > 0 {
				list[i] = receiver.Name + "<" + receiver.Address + ">"
			} else {
				list[i] = receiver.Address
			}
		}
		jmail.to = strings.Join(list, ",")
	}
	if subject, err := header.Subject(); err == nil {
		log.Println("Subject:", subject)
		jmail.subject = subject
	}

	htmlBody, plainBody := "", ""
	_ = htmlBody
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Println(err.Error())
			WriteLog(critical, "#18 getMailContent nextPart err: "+err.Error())
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			bodycontent := string(b)

			if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") {
				htmlBody = bodycontent
				continue
			}

			if strings.HasPrefix(p.Header.Get("Content-Type"), "text/plain") {
				plainBody = bodycontent
				continue
			}

			plainBody = bodycontent
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			jmail.attachment += string(filename) + "\r\n"
		}
	}
	isEnabled, eror := isHTMLenabled(roomID)
	if eror != nil {
		WriteLog(critical, "#55 isHTMLenabled: "+eror.Error())
	}
	if len(htmlBody) > 0 && isEnabled {
		jmail.body = html.UnescapeString(htmlBody)
		jmail.htmlFormat = true
	} else {
		if len(strings.Trim(plainBody, " ")) == 0 {
			plainBody = htmlBody
		}
		parseMailBody(&plainBody)
		jmail.body = plainBody
		jmail.htmlFormat = false
	}

	return &jmail
}

func parseMailBody(body *string) {
	*body = strings.ReplaceAll(*body, "<br>", "\r\n")
	*body = strip.StripTags(html.UnescapeString(*body))
}

func viewMailbox(roomID string, client *mautrix.Client) {
	imapAccID, _, erro := getRoomAccounts(roomID)
	if erro != nil {
		WriteLog(critical, "#50 getRoomAccounts: "+erro.Error())
		client.SendText(id.RoomID(roomID), "An server-error occured Errorcode: #50")
		return
	}
	if imapAccID != -1 {
		mailbox, err := getMailbox(roomID)
		if err != nil {
			WriteLog(critical, "#51 getMailbox: "+err.Error())
			client.SendText(id.RoomID(roomID), "An server-error occured Errorcode: #51")
			return
		}
		client.SendText(id.RoomID(roomID), "The current mailbox for this room is: "+mailbox)
	} else {
		client.SendText(id.RoomID(roomID), "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
	}
}

func viewMailboxes(roomID string, client *mautrix.Client) {
	imapAccID, _, erro := getRoomAccounts(roomID)
	if erro != nil {
		WriteLog(critical, "#48 getRoomAccounts: "+erro.Error())
		client.SendText(id.RoomID(roomID), "An server-error occured Errorcode: #48")
		return
	}
	if imapAccID != -1 {
		mailboxes, err := getMailboxes(clients[roomID])
		if err != nil {
			WriteLog(critical, "#47 getMailboxes: "+err.Error())
			client.SendText(id.RoomID(roomID), "An server-error occured Errorcode: #47")
			return
		}
		client.SendText(id.RoomID(roomID), "Your mailboxes:\r\n"+mailboxes+"\r\nUse !setmailbox <mailbox> to change your mailbox")
	} else {
		client.SendText(id.RoomID(roomID), "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
	}
}

func viewBlocklist(roomID string, client *mautrix.Client) {
	imapAccID, _, erro := getRoomAccounts(roomID)
	if erro != nil {
		WriteLog(critical, "#48 getRoomAccounts: "+erro.Error())
		client.SendText(id.RoomID(roomID), "An server-error occured Errorcode: #48")
		return
	}
	if imapAccID != -1 {
		blocklist := getBlocklist(imapAccID)
		var msg string
		if len(blocklist) > 0 {
			msg = "Blocked addresses:\n"
			for _, blo := range blocklist {
				msg += "> " + blo + "\n"
			}
		} else {
			msg = "No addresses blocked!"
		}
		client.SendText(id.RoomID(roomID), msg)
	} else {
		client.SendText(id.RoomID(roomID), "You have to setup an IMAP account to use this command. Use !setup or !login for more informations")
	}
}
