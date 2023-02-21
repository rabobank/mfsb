package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/rabobank/mfsb/conf"
)

func GetDB() (db *sql.DB) {
	dbDriver := "mysql"
	db, err := sql.Open(dbDriver, fmt.Sprintf("%s:%s@(%s)/%s?parseTime=true", conf.BrokerDBUser, conf.BrokerDBPassword, conf.BrokerDBHost, conf.BrokerDBName))
	if err != nil {
		panic(err.Error())
	}
	return db
}
