package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/viper"
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

type dbChange struct {
	version int
	changes string
}

var tables = []table{
	{"mail", "mail TEXT, room INTEGER"},
	{"rooms", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, imapAccount INTEGER DEFAULT -1, smtpAccount INTEGER DEFAULT -1, mailCheckInterval INTEGER, isHTMLenabled INTEGER"},
	{"imapAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, username TEXT, password TEXT, ignoreSSL INTEGER, mailbox TEXT"},
	{"smtpAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, port int, username TEXT, password TEXT, ignoreSSL INTEGER"},
	{"emailWritingTemp", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, receiver TEXT, subject TEXT DEFAULT ' ', body TEXT DEFAULT ' ', markdown INTEGER"},
	{"version", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, version INTEGER"},
	{"emailAttachments", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, writeTempID INTEGER, fileName TEXT"},
}

func handleDBVersion() {
	countVersions, err := dbCountVersion()
	if err != nil {
		panic(err)
	}
	if countVersions == 1 {
		vers, err := readVersion()
		if err != nil {
			WriteLog(logError, "#59 countVersions=1 mail: "+err.Error())
			log.Fatal("An critical error accoured. Try to restart")
			return
		}
		if vers != version {
			startDBupgrader(vers)
		}
	} else if countVersions == 0 {
		err := saveVersion(1)
		if err != nil {
			WriteLog(logError, "#60 countVersions=0 mail: "+err.Error())
			log.Fatal("An critical error accoured. Try to restart")
			return
		}
		startDBupgrader(1)
	} else {
		WriteLog(logError, "#61 countVersions>1 mail")
		log.Fatal("An critical error accoured. Try to restart")
	}
}

func saveVersion(version int) error {
	_, err := db.Exec("DELETE FROM version")
	if err != nil {
		WriteLog(logError, "#62 countVersions mail: "+err.Error())
		log.Fatal("An critical error accoured. Try to restart")
		return err
	}
	stmt, err := db.Prepare("INSERT INTO version (version) VALUES(?)")
	if err != nil {
		WriteLog(logError, "#57 countVersions mail: "+err.Error())
		log.Fatal("An critical error accoured. Try to restart")
		return err
	}
	_, err = stmt.Exec(version)
	if err != nil {
		WriteLog(logError, "#58 countVersionsExec mail: "+err.Error())
		log.Fatal("An critical error accoured. Try to restart")
		return err
	}
	return nil
}

var dbChanges = []dbChange{
	{2, "ALTER TABLE rooms ADD isHTMLenabled INTEGER"},
	{2, "UPDATE rooms SET isHTMLenabled=0"},
	{7, "CREATE TABLE `blocklist` (`pkID` INTEGER PRIMARY KEY AUTOINCREMENT, `imapAccount` INTEGER, `address` INTEGER);"},
}

func startDBupgrader(oldVers int) {
	WriteLog(info, "starting db upgrade from version "+strconv.Itoa(oldVers)+" to "+strconv.Itoa(version))
	var err error
	for _, change := range dbChanges {
		if change.version > oldVers {
			fmt.Println("Update Database:", change.changes)
			_, err = db.Exec(change.changes)
		}
	}
	if err != nil {
		WriteLog(critical, "#63 Error upgrading db: "+err.Error())
	}
	err = saveVersion(version)
	if err != nil {
		WriteLog(critical, "#64 Error upgrading/saveVersion db: "+err.Error())
		return
	}
}

func readVersion() (vers int, err error) {
	stmt, err := db.Prepare("SELECT version FROM version")
	if err != nil {
		return 0, err
	}

	err = stmt.QueryRow().Scan(&vers)
	if err != nil {
		return 0, err
	}
	return vers, nil
}

func dbCountVersion() (int, error) {
	stmt, err := db.Prepare("SELECT COUNT(version) FROM version")
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow().Scan(&count)
	if err != nil {
		return -1, err
	}
	return count, nil
}

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

func deleteAttachments(roomID string) {
	stmt, err := db.Prepare("SELECT pk_id FROM emailWritingTemp WHERE roomID=?")
	if err == nil {
		var pkid int
		err = stmt.QueryRow(roomID).Scan(&pkid)
		if err == nil {
			attachments, err := getAttachments(pkid)
			if err == nil {
				for _, i := range attachments {
					deleteTempFile(i)
				}
			}
			stmt, err = db.Prepare("DELETE FROM emailAttachments WHERE writeTempID=?")
			if err == nil {
				stmt.Exec(pkid)
			}
		}
	}
}

func deleteWritingTemp(roomID string) error {
	deleteAttachments(roomID)
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

func addEmailAttachment(emailid int, filename string) error {
	stmt, err := db.Prepare("INSERT INTO emailAttachments (writeTempID, fileName) VALUES(?,?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(emailid, filename)
	return err
}

func deleteAttachment(fileName string, writeTempID int) error {
	stmt, err := db.Prepare("DELETE FROM emailAttachments WHERE fileName=? AND writeTempID=?")
	if err != nil {
		return err
	}
	stmt.Exec(fileName, writeTempID)
	return nil
}

func getAttachments(writingTempID int) ([]string, error) {
	rows, err := db.Query("SELECT fileName FROM emailAttachments WHERE writeTempID=?", writingTempID)
	if err != nil {
		return nil, err
	}

	var attachments []string
	var name string
	for rows.Next() {
		rows.Scan(&name)
		attachments = append(attachments, name)
	}
	return attachments, nil
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

	stmt4, err := db.Prepare("DELETE FROM smtpAccounts WHERE pk_id=(SELECT smtpAccount FROM rooms WHERE roomID=?)")
	checkErr(err)
	stmt4.Exec(roomID)

	deleteMails(roomID)

	stmt2, err := db.Prepare("DELETE FROM rooms WHERE roomID=?")
	checkErr(err)
	stmt2.Exec(roomID)
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
	stmt, err := db.Prepare("INSERT INTO rooms (roomID, mailCheckInterval, isHTMLenabled) VALUES(?,?,?)")
	checkErr(err)

	isenabled := 0
	if viper.GetBool("htmlDefault") {
		isenabled = 1
	}

	res, err := stmt.Exec(roomID, mailCheckInterval, isenabled)
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
			fmt.Println(berr.Error())
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
		fmt.Println(berr.Error())
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
		fmt.Println(berr.Error())
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

func isHTMLenabled(roomID string) (bool, error) {
	stmt, err := db.Prepare("SELECT isHTMLenabled FROM rooms WHERE roomID=?")
	if err != nil {
		return false, err
	}
	var isEnabled int
	err = stmt.QueryRow(roomID).Scan(&isEnabled)
	if err != nil {
		return false, err
	}
	ishtml := true
	if isEnabled == 0 {
		ishtml = false
	}
	return ishtml, nil
}

func setHTMLenabled(roomID string, enabled bool) error {
	stmt, err := db.Prepare("UPDATE rooms SET isHTMLenabled=? WHERE roomID=?")
	if err != nil {
		return err
	}
	isenabled := 1
	if !enabled {
		isenabled = 0
	}
	_, err = stmt.Exec(isenabled, roomID)
	if err != nil {
		return err
	}
	return nil
}

func getBlocklist(imapAccount int) []string {
	rows, err := db.Query("SELECT address FROM blocklist WHERE imapAccount=?", imapAccount)
	if err != nil {
		fmt.Println("Err:", err.Error())
		return []string{"Error fetching accounts:" + err.Error()}
	}
	var blocklist []string
	var address string
	for rows.Next() {
		rows.Scan(&address)
		blocklist = append(blocklist, address)
	}
	return blocklist
}

func isInBlocklist(imapAccount int, addr string) bool {
	row := db.QueryRow("SELECT COUNT(*) FROM blocklist WHERE imapAccount=? AND address=?", imapAccount, addr)
	var has int
	row.Scan(&has)
	return has == 1
}

func addEmailToBlocklist(imapAcc int, emailaddr string) error {
	if isInBlocklist(imapAcc, emailaddr) {
		return nil
	}
	stmt, err := db.Prepare("INSERT INTO blocklist (imapAccount, address) VALUES(?,?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(imapAcc, emailaddr)
	return err
}

func clearBlocklist(imapAcc int) error {
	stmt, err := db.Prepare("DELETE FROM blocklist WHERE imapAccount=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(imapAcc)
	return err
}

func removeEmailFromBlocklist(imapAcc int, addr string) error {
	if !isInBlocklist(imapAcc, addr) {
		return errors.New("not on blocklist")
	}
	stmt, err := db.Prepare("DELETE FROM blocklist WHERE imapAccount=? AND address=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(imapAcc, addr)
	return err
}

//returns true if email matches blocklist
func checkForBlocklist(roomID string, senderEmail string) bool {
	acc, _, err := getRoomAccounts(roomID)
	if err != nil {
		fmt.Println("Err:", err.Error())
		return false
	}
	blocklist := getBlocklist(acc)
	for _, bll := range blocklist {
		if strings.HasPrefix(bll, "*") || strings.HasSuffix(bll, "*") {
			if strings.HasPrefix(bll, "*") && !strings.HasSuffix(bll, "*") {
				return strings.HasSuffix(senderEmail, bll[1:])
			} else if !strings.HasPrefix(bll, "*") && strings.HasSuffix(bll, "*") {
				return strings.HasPrefix(senderEmail, bll[:len(bll)-1])
			} else {
				//has both
				return strings.HasPrefix(senderEmail, bll[1:len(bll)-1]) && strings.HasSuffix(senderEmail, bll[1:len(bll)-1])
			}
		} else {
			if bll == senderEmail {
				return true
			}
		}
	}
	return false
}
