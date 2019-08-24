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
		log.Fatal(err)
	}
	if mbox.Messages == 0 {
		log.Fatal("No message in mailbox")
	}

	seqSet := new(imap.SeqSet)
	for i := uint32(0); i < 10; i++ {
		currNum := mbox.Messages - i
		if currNum < 0 {
			break
		}
		seqSet.AddNum(mbox.Messages - i)
	}

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, section.FetchItem()}
	go func() {
		if err := mClient.Fetch(seqSet, items, messages); err != nil {
			log.Fatal(err)
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
		return nil
	}

	r := msg.GetBody(section)
	if r == nil {
		fmt.Println("reader is nli")
		return nil
	}

	jmail := email{}

	mr, err := mail.CreateReader(r)
	if err != nil {
		fmt.Println(err.Error())
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
