package main

import "log"

func insertEmail(email string) error {
	if val, err := dbContainsMail(email); val && err == nil {
		return nil
	} else if err != nil {
		return err
	}

	stmt, err := db.Prepare("INSERT OR IGNORE INTO mails(mail) values(?)")
	checkErr(err)

	res, err := stmt.Exec(email)
	checkErr(err)
	_ = res
	return nil
}

func dbContainsMail(mail string) (bool, error) {
	stmt, err := db.Prepare("SELECT COUNT(mail) FROM mails WHERE mail=?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(mail).Scan(&count)
	if err != nil {
		return false, err
	}
	return (count > 0), nil
}

func createTable() error {
	res, err := db.Exec("CREATE TABLE IF NOT EXISTS mails (mail TEXT)")
	_ = res
	return err
}

func checkErr(de error) {
	if de != nil {
		log.Fatal(de)
	}
}
