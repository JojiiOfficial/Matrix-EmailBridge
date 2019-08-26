package main

import (
	"encoding/base64"
	"fmt"
	"log"
)

type table struct {
	name, values string
}

type emailTemp struct {
	pkID                            int
	roomID, receiver, subject, body string
	markdown                        bool
}

type imapAccountount struct {
	host, username, password, roomID, mailbox string
	ignoreSSL                                 bool
	roomPKID, mailCheckInterval               int
	silence                                   bool
}

type smtpAccount struct {
	host, username, password, roomID string
	ignoreSSL                        bool
	roomPKID, port, pk               int
}

var tables = []table{
	table{"mail", "mail TEXT, room INTEGER"},
	table{"rooms", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, imapAccount INTEGER DEFAULT -1, smtpAccount INTEGER DEFAULT -1, mailCheckInterval INTEGER"},
	table{"imapAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, username TEXT, password TEXT, ignoreSSL INTEGER, mailbox TEXT"},
	table{"smtpAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, port int, username TEXT, password TEXT, ignoreSSL INTEGER"},
	table{"emailWritingTemp", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, receiver TEXT, subject TEXT DEFAULT ' ', body TEXT DEFAULT ' ', markdown INTEGER"}}

func insertEmail(email string, roomPK int) error {
	if val, err := dbContainsMail(email, roomPK); val && err == nil {
		return nil
	} else if err != nil {
		return err
	}

	stmt, err := db.Prepare("INSERT OR IGNORE INTO mail(mail, room) values(?,?)")
	checkErr(err)

	_, err = stmt.Exec(email, roomPK)
	checkErr(err)
	return nil
}

func dbContainsMail(mail string, roomPK int) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(mail) FROM mail WHERE mail=? AND room=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(mail, roomPK).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func deleteWritingTemp(roomID string) error {
	stmt, err := db.Prepare("DELETE FROM emailWritingTemp WHERE roomID=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(roomID)
	return err
}

func deleteAllWritingTemps() error {
	_, err := db.Exec("DELETE FROM emailWritingTemp")
	return err
}

func newWritingTemp(roomID, receiver string) error {
	stmt, err := db.Prepare("INSERT INTO emailWritingTemp (roomID, receiver) VALUES(?,?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(roomID, receiver)
	return err
}

func getWritingTemp(roomID string) (*emailTemp, error) {
	stmt, err := db.Prepare("SELECT pk_id, roomID, receiver, subject, body, markdown FROM emailWritingTemp WHERE roomID=?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var pkID, markdown int
	var rID, receiver, subject, body string
	err = stmt.QueryRow(roomID).Scan(&pkID, &rID, &receiver, &subject, &body, &markdown)
	if err != nil {
		return nil, err
	}
	mrkdwn := false
	if markdown == 1 {
		mrkdwn = true
	}
	return &emailTemp{pkID, rID, receiver, subject, body, mrkdwn}, nil
}

func saveWritingtemp(roomID, key, value string) error {
	stmt, err := db.Prepare("UPDATE emailWritingTemp SET " + key + "=? WHERE roomID=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(value, roomID)
	return err
}

func isUserWritingEmail(roomID string) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(*) FROM emailWritingTemp WHERE roomID=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(roomID).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func deleteRoomAndEmailByRoomID(roomID string) {
	stmt1, err := db.Prepare("DELETE FROM imapAccounts WHERE pk_id=(SELECT imapAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt1.Exec(roomID)

	deleteMails(roomID)

	stmt2, err := db.Prepare("DELETE FROM rooms WHERE roomID=?")
	checkErr(err)
	stmt2.Exec(roomID)

	stmt4, err := db.Prepare("DELETE FROM smtpAccounts WHERE pk_id=(SELECT smtpAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt4.Exec(roomID)
}

func deleteMails(roomID string) {
	stmt3, err := db.Prepare("DELETE FROM mail WHERE room=(SELECT pk_id FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt3.Exec(roomID)
}

func createTable(name, values string) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + name + " (" + values + ")")
	checkErr(err)
	return err
}

func createAllTables() {
	for _, tab := range tables {
		createTable(tab.name, tab.values)
	}
}

func getRoomInfo(roomID string) (string, error) {
	stmt, err := db.Prepare("SELECT IFNULL(imapAccounts.username, 'not configured'), IFNULL(smtpAccounts.username, 'not configured') FROM rooms LEFT JOIN imapAccounts ON (imapAccounts.pk_id = rooms.imapAccount) LEFT JOIN smtpAccounts ON (smtpAccounts.pk_id = rooms.smtpAccount)  WHERE roomID=?")
	if err != nil {
		return "", err
	}
	defer stmt.Close()
	var imapAccount, smtpAccount string
	err = stmt.QueryRow(roomID).Scan(&imapAccount, &smtpAccount)
	if err != nil {
		return "", err
	}

	infoText := "IMAP account: " + imapAccount + "\r\nSMTP account: " + smtpAccount

	return infoText, nil
}

func getRoomPKID(roomID string) (int, error) {
	stmt, err := db.Prepare("SELECT pk_id FROM rooms WHERE roomID=?")
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	var id int
	err = stmt.QueryRow(roomID).Scan(&id)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func getRoomAccounts(roomID string) (imapAccount, smtpAccount int, err error) {
	imapAccount = -1
	smtpAccount = -1
	stmt, err := db.Prepare("SELECT imapAccount, smtpAccount FROM rooms WHERE roomID=?")
	if err != nil {
		return
	}
	defer stmt.Close()
	err = stmt.QueryRow(roomID).Scan(&imapAccount, &smtpAccount)
	if err != nil {
		err = nil
		return
	}
	return
}

func insertNewRoom(roomID string, mailCheckInterval int) int64 {
	stmt, err := db.Prepare("INSERT INTO rooms (roomID, mailCheckInterval) VALUES(?,?)")
	checkErr(err)
	res, err := stmt.Exec(roomID, mailCheckInterval)
	if err != nil {
		WriteLog(critical, "#19 insertNewRoom could not execute err: "+err.Error())
		return -1
	}
	id, _ := res.LastInsertId()
	return id
}

func saveImapAcc(roomID string, account int) error {
	stmt, err := db.Prepare("UPDATE rooms SET imapAccount=? WHERE roomID=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(account, roomID)
	if err != nil {
		return err
	}
	return nil
}

func saveSMTPAcc(roomID string, account int) error {
	stmt, err := db.Prepare("UPDATE rooms SET smtpAccount=? WHERE roomID=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(account, roomID)
	if err != nil {
		return err
	}
	return nil
}

func removeSMTPAccount(roomID string) {
	stmt4, err := db.Prepare("DELETE FROM smtpAccounts WHERE pk_id=(SELECT smtpAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt4.Exec(roomID)

	stmt1, err := db.Prepare("UPDATE rooms SET smtpAccount=-1 WHERE roomID=?")
	checkErr(err)
	stmt1.Exec(roomID)
}

func hasRoom(roomID string) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(pk_id) FROM rooms WHERE roomID=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(roomID).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func insertSMTPAccountount(host string, port int, username, password string, ignoreSSL bool) (id int64, err error) {
	id = -1
	stmt, err := db.Prepare("INSERT INTO smtpAccounts (host, port, username, password, ignoreSSL) VALUES(?,?,?,?,?)")
	if !checkErr(err) {
		WriteLog(critical, "#31 insertimapAccountount could not execute err: "+err.Error())
		return
	}
	ign := 0
	if ignoreSSL {
		ign = 1
	}
	a, er := stmt.Exec(host, port, username, base64.StdEncoding.EncodeToString([]byte(password)), ign)
	if !checkErr(er) {
		WriteLog(critical, "#32 insertimapAccountount could not execute err: "+err.Error())
		return
	}
	id, e := a.LastInsertId()
	if !checkErr(e) {
		WriteLog(critical, "#33 insertimapAccountount could not get lastID err: "+err.Error())
	}
	return id, nil
}

func insertimapAccountount(host, username, password, mailbox string, ignoreSSl bool) (id int64, success bool) {
	stmt, err := db.Prepare("INSERT INTO imapAccounts (host, username, password, ignoreSSL, mailbox) VALUES(?,?,?,?,?)")
	success = true
	if !checkErr(err) {
		WriteLog(critical, "#20 insertimapAccountount could not execute err: "+err.Error())
		success = false
	}
	ign := 0
	if ignoreSSl {
		ign = 1
	}
	a, er := stmt.Exec(host, username, base64.StdEncoding.EncodeToString([]byte(password)), ign, mailbox)
	if !checkErr(er) {
		WriteLog(critical, "#21 insertimapAccountount could not execute err: "+err.Error())
		success = false
	}
	id, e := a.LastInsertId()
	if !checkErr(e) {
		WriteLog(critical, "#22 insertimapAccountount could not get lastID err: "+err.Error())
		success = false
	}
	return id, success
}

func checkErr(de error) bool {
	if de != nil {
		log.Fatal(de)
		return false
	}
	return true
}

func isImapAccountAlreadyInUse(email string) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(pk_id) FROM imapAccounts WHERE username=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(email).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func isSMTPAccountAlreadyInUse(email string) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(pk_id) FROM smtpAccounts WHERE username=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(email).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func getimapAccounts() ([]imapAccountount, error) {
	rows, err := db.Query("SELECT host, username, password, ignoreSSL, rooms.roomID, rooms.pk_id, rooms.mailCheckInterval, mailbox FROM imapAccounts INNER JOIN rooms ON (rooms.imapAccount = imapAccounts.pk_id)")
	if err != nil {
		return nil, err
	}

	var list []imapAccountount
	var host, username, password, roomID, mailbox string
	var ignoreSSL, roomPKID, mailCheckInterval int
	for rows.Next() {
		rows.Scan(&host, &username, &password, &ignoreSSL, &roomID, &roomPKID, &mailCheckInterval, &mailbox)
		ignssl := false
		if ignoreSSL == 1 {
			ignssl = true
		}
		pass, berr := base64.StdEncoding.DecodeString(password)
		if berr != nil {
			fmt.Println(err.Error())
			continue
		}
		list = append(list, imapAccountount{host, username, string(pass), roomID, mailbox, ignssl, roomPKID, mailCheckInterval, false})
	}
	return list, nil
}

func getIMAPAccount(roomID string) (*imapAccountount, error) {
	var host, username, password, rid, mailbox string
	var ignoreSSL, roomPKID, mailCheckInterval int

	res, err := db.Prepare("SELECT host, username, password, ignoreSSL, rooms.roomID, rooms.pk_id, rooms.mailCheckInterval, mailbox FROM imapAccounts INNER JOIN rooms ON (rooms.imapAccount = imapAccounts.pk_id) WHERE rooms.roomID=?")

	if err != nil {
		return nil, err
	}

	err = res.QueryRow(roomID).Scan(&host, &username, &password, &ignoreSSL, &rid, &roomPKID, &mailCheckInterval, &mailbox)

	if err != nil {
		return nil, err
	}
	ignssl := false
	if ignoreSSL == 1 {
		ignssl = true
	}
	pass, berr := base64.StdEncoding.DecodeString(password)

	if berr != nil {
		fmt.Println(err.Error())
		return nil, berr
	}

	return &imapAccountount{host, username, string(pass), roomID, mailbox, ignssl, roomPKID, mailCheckInterval, false}, nil
}

func getSMTPAccount(roomID string) (*smtpAccount, error) {
	rows, err := db.Prepare("SELECT smtpAccounts.pk_id, host, port, username, password, rooms.pk_id, ignoreSSL FROM smtpAccounts INNER JOIN rooms ON (rooms.smtpAccount = smtpAccounts.pk_id) WHERE rooms.roomID=?")
	if err != nil {
		return nil, err
	}

	var host, username, password string
	var ignoreSSL, roomPKID, pk, port int
	err = rows.QueryRow(roomID).Scan(&pk, &host, &port, &username, &password, &roomPKID, &ignoreSSL)
	if err != nil {
		return nil, err
	}
	var ignSSL = false
	if ignoreSSL == 1 {
		ignSSL = true
	}
	pass, berr := base64.StdEncoding.DecodeString(password)
	if berr != nil {
		fmt.Println(err.Error())
		return nil, berr
	}
	return &smtpAccount{host, username, string(pass), roomID, ignSSL, roomPKID, port, pk}, nil
}

func saveMailbox(roomID, newMailbox string) error {
	stmt, err := db.Prepare("UPDATE imapAccounts SET mailbox=? WHERE pk_id=(SELECT imapAccount FROM rooms WHERE roomID=?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(newMailbox, roomID)
	return err
}

func getMailbox(roomID string) (string, error) {
	stmt, err := db.Prepare("SELECT mailbox FROM imapAccounts WHERE pk_id=(SELECT imapAccount FROM rooms WHERE roomID=?)")
	if err != nil {
		return "", err
	}
	mailbox := ""
	err = stmt.QueryRow(roomID).Scan(&mailbox)
	return mailbox, err
}
