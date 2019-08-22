package main

import (
	"crypto/tls"
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

func loginMail(host, username, password string) error {
	ailClient, err := client.DialTLS(host, &tls.Config{InsecureSkipVerify: true})
	mailClient = ailClient

	if err != nil {
		return err
	}

	if err := ailClient.Login(username, password); err != nil {
		return err
	}

	return nil
}

func getMails(messages chan *imap.Message) *imap.BodySectionName {
	mbox, err := mailClient.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}
	if mbox.Messages == 0 {
		log.Fatal("No message in mailbox")
	}

	seqSet := new(imap.SeqSet)
	for i := uint32(0); i < 10; i++ {
		seqSet.AddNum(mbox.Messages - i)
	}

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, section.FetchItem()}
	go func() {
		if err := mailClient.Fetch(seqSet, items, messages); err != nil {
			log.Fatal(err)
		}
	}()
	return section
}

type email struct {
	body, from, to, subject, attachment string
	date                                time.Time
}

func getMailContent(msg *imap.Message, section *imap.BodySectionName) email {
	if msg == nil {
		log.Fatal("Server didn't returned message")
	}

	r := msg.GetBody(section)
	if r == nil {
		log.Fatal("Server didn't returned message body")
	}

	jmail := email{}

	mr, err := mail.CreateReader(r)
	if err != nil {
		log.Fatal(err)
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

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			jmail.body += string(b)
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			jmail.attachment += string(filename)
		}
	}
	return jmail
}

func parseMailBody(body *string) {
	*body = strings.ReplaceAll(strip.StripTags(html.UnescapeString(*body)), "\n\n", "\n")
}
