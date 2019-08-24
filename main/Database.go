package main

import "log"

type table struct {
	name, values string
}

var tables = []table{
	table{"mail", "mail TEXT, room INTEGER"},
	table{"rooms", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, roomID TEXT, imapAccount INTEGER DEFAULT -1, smtpAccount INTEGER DEFAULT -1, mailCheckInterval INTEGER"},
	table{"imapAccounts", "pk_id INTEGER PRIMARY KEY AUTOINCREMENT, host TEXT, username TEXT, password TEXT, ignoreSSL INTEGER"}}

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

	stmt2, err := db.Prepare("DELETE FROM rooms WHERE roomID=?")
	checkErr(err)
	stmt2.Exec(roomID)
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
	stmt, err := db.Prepare("SELECT imapAccounts.username FROM rooms LEFT JOIN imapAccounts ON (imapAccounts.pk_id = rooms.imapAccount) WHERE roomID=?")
	if err != nil {
		return "", err
	}
	defer stmt.Close()
	var imapAccount, smtpAccount string
	_ = smtpAccount
	err = stmt.QueryRow(roomID).Scan(&imapAccount)
	if err != nil {
		return "", err
	}

	infoText := "IMAP account: " + imapAccount + "\r\nSMTP account: " + "not implemented yet"

	return infoText, nil
}

func insertNewRoom(roomID string, imapAccountID, mailCheckInterval int) int64 {
	stmt, err := db.Prepare("INSERT INTO rooms (roomID, imapAccount, mailCheckInterval) VALUES(?,?,?)")
	checkErr(err)
	res, err := stmt.Exec(roomID, imapAccountID, mailCheckInterval)
	if err != nil {
		log.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
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

func insertimapAccountount(host, username, password string, ignoreSSl bool) (id int64, success bool) {
	stmt, err := db.Prepare("INSERT INTO imapAccounts (host, username, password, ignoreSSL) VALUES(?,?,?,?)")
	success = true
	if !checkErr(err) {
		success = false
	}
	ign := 0
	if ignoreSSl {
		ign = 1
	}
	a, er := stmt.Exec(host, username, password, ign)
	if !checkErr(er) {
		success = false
	}
	id, e := a.LastInsertId()
	if !checkErr(e) {
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
	host, username, password, roomID string
	ignoreSSL                        bool
	roomPKID, mailCheckInterval      int
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

func getimapAccounts() ([]imapAccountount, error) {
	rows, err := db.Query("SELECT host, username, password, ignoreSSL, rooms.roomID, rooms.pk_id, rooms.mailCheckInterval FROM imapAccounts INNER JOIN rooms ON (rooms.imapAccount = imapAccounts.pk_id)")
	if !checkErr(err) {
		return nil, err
	}

	var list []imapAccountount
	var host, username, password, roomID string
	var ignoreSSL, roomPKID, mailCheckInterval int
	for rows.Next() {
		rows.Scan(&host, &username, &password, &ignoreSSL, &roomID, &roomPKID, &mailCheckInterval)
		ignssl := false
		if ignoreSSL == 1 {
			ignssl = true
		}
		list = append(list, imapAccountount{host, username, password, roomID, ignssl, roomPKID, mailCheckInterval})
	}
	return list, nil
}
