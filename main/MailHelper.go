package main

import (
	"crypto/tls"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/mail"
	strip "github.com/grokify/html-strip-tags-go"
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

func getMails(mClient *client.Client, messages chan *imap.Message) *imap.BodySectionName {
	mbox, err := mClient.Select("INBOX", false)
	if err != nil {
		WriteLog(logError, "#12 couldnt get INBOX "+err.Error())
		return nil
	}

	if mbox == nil {
		WriteLog(logError, "#23 getMails mbox is nli")
		return nil
	}

	if mbox.Messages == 0 {
		WriteLog(logError, "#13 getMails no messages in inbox ")
		return nil
	}

	seqSet := new(imap.SeqSet)
	maxMessages := uint32(10)
	if mbox.Messages < 10 {
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
	return section
}

type email struct {
	body, from, to, subject, attachment string
	date                                time.Time
}

func getMailContent(msg *imap.Message, section *imap.BodySectionName) *email {
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
		log.Println("From:", from)
		for i := 0; i < len(from); i++ {
			jmail.from += from[i].String() + ", "
		}
		jmail.from = jmail.from[:len(jmail.from)-2]
	}
	if to, err := header.AddressList("To"); err == nil {
		log.Println("To:", to)
		for i := 0; i < len(to); i++ {
			jmail.to += to[i].String() + ", "
		}
		jmail.to = jmail.to[:len(jmail.to)-2]
	}
	if subject, err := header.Subject(); err == nil {
		log.Println("Subject:", subject)
		jmail.subject = subject
	}

	hadPlan := false
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
			if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") && hadPlan {
				continue
			}

			if strings.HasPrefix(p.Header.Get("Content-Type"), "text/plain") {
				hadPlan = true
			}
			b, _ := ioutil.ReadAll(p.Body)
			bodycontent := string(b)
			parseMailBody(&bodycontent)
			jmail.body = bodycontent
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			jmail.attachment += string(filename)
		}
	}
	return &jmail
}

func parseMailBody(body *string) {
	*body = strip.StripTags(html.UnescapeString(*body))
	*body = strings.TrimRight(*body, "\r\n")
	*body = strings.TrimLeft(*body, "\r\n")
	*body = strings.ReplaceAll(*body, "\r\n\r\n", "\n")
}
