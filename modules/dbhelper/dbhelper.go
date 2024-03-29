//数据库接口模块
//author：gzf
package dbhelper

import (
	"database/sql"
	"strings"

	"github.com/Snipergzf/MessageHive/modules/onlinetable"
	_ "github.com/go-sql-driver/mysql"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

type Config struct {
	onlinetable *onlinetable.Container
}

func NewConfig(onlinetable *onlinetable.Container) Config {
	return Config{
		onlinetable: onlinetable,
	}
}

func InsertGroupEntity(config Config) error {
	db, err := sql.Open("mysql", "dhc:denghc@/Register")
	if err != nil {
		return err
	}

	defer db.Close()

	err = db.Ping()
	if err != nil {
		return err
	}

	rows, err := db.Query("select * from groups")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		var group_id string
		var group_member string
		err = rows.Scan(&id, &group_id, &group_member)
		if err != nil {
			return err
		}
		config.onlinetable.AddGroupEntity(group_id, strings.Split(group_member, ";"), false)
		log.Debug("Group entity group_id: %s added", group_id)
	}
	return nil
}
