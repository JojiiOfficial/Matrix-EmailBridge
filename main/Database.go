package main

import (
	"encoding/base64"
	"fmt"
	"log"
)

type table struct {
	name, values string
}

var tables = []table{
	table{"mail", "mail TEXT, room INTEGER"},
	table{"rooms", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, imapAccount INTEGER DEFAULT -1, smtpAccount INTEGER DEFAULT -1, mailCheckInterval INTEGER"},
	table{"imapAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, username TEXT, password TEXT, ignoreSSL INTEGER, mailbox TEXT"},
	table{"smtpAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, port int, username TEXT, password TEXT, ignoreSSL INTEGER"}}

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

func deleteRoomAndEmailByRoomID(roomID string) {
	stmt1, err := db.Prepare("DELETE FROM imapAccounts WHERE pk_id=(SELECT imapAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt1.Exec(roomID)

	stmt3, err := db.Prepare("DELETE FROM mail WHERE room=(SELECT pk_id FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt3.Exec(roomID)

	stmt2, err := db.Prepare("DELETE FROM rooms WHERE roomID=?")
	checkErr(err)
	stmt2.Exec(roomID)

	stmt4, err := db.Prepare("DELETE FROM smtpAccounts WHERE pk_id=(SELECT smtpAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt4.Exec(roomID)
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
	stmt, err := db.Prepare("SELECT imapAccounts.username, smtpAccounts.username FROM rooms LEFT JOIN imapAccounts ON (imapAccounts.pk_id = rooms.imapAccount) LEFT JOIN smtpAccounts ON (smtpAccounts.pk_id = rooms.smtpAccount)  WHERE roomID=?")
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

type imapAccountount struct {
	host, username, password, roomID, mailbox string
	ignoreSSL                                 bool
	roomPKID, mailCheckInterval               int
	silence                                   bool
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
	if !checkErr(err) {
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
