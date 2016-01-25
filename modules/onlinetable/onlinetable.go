// 在线表模块
package onlinetable

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Snipergzf/MessageHive/modules/message"
	_ "github.com/go-sql-driver/mysql"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

// 实体类型定义
const (
	ENTITY_TYPE_USER = iota
	ENTITY_TYPE_GROUP
)

//群成员操作类型定义
const (
	ADD_GROUP_MEMBER = "add"
	DEL_GROUP_MEMBER = "delete"
)

// 实体结构
type Entity struct {
	Uid       string
	Type      int
	Pipe      chan *message.Container
	List      []string
	LoginTime time.Time
}

// 在线表结构
type Container struct {
	sync.RWMutex                    // 同步锁
	storage      map[string]*Entity // 哈希表
}

var instance *Container

var initctx sync.Once

func NewContainer() *Container {
	initctx.Do(func() {
		instance = new(Container)
		instance.storage = make(map[string]*Entity)
	})
	return instance
}

// 通过UID获取实体
func (ct Container) GetEntity(uid string) (*Entity, error) {
	ct.RLock()
	if entity, ok := ct.storage[uid]; ok {
		ct.RUnlock()
		return entity, nil
	}
	ct.RUnlock()
	return new(Entity), errors.New("Entity not found")
}

// 向在线表中添加实体
func (ct *Container) AddEntity(uid string, pipe chan *message.Container) error {
	ct.Lock()
	delete(ct.storage, uid)
	entity := &Entity{Uid: uid, Type: ENTITY_TYPE_USER, Pipe: pipe, LoginTime: time.Now().UTC()}
	ct.storage[uid] = entity
	ct.Unlock()
	log.Debug("Entity uid: %s added", uid)
	return nil
}

// 向在线表中添加群组实体
func (ct *Container) AddGroupEntity(uid string, uidlist []string) error {
	//TODO: Ensure every user in group is existed
	ct.Lock()
	entity := &Entity{Uid: uid, Type: ENTITY_TYPE_GROUP, List: uidlist, LoginTime: time.Now().UTC()}
	ct.storage[uid] = entity
	str := strings.Join(uidlist, ";")
	db, err := sql.Open("mysql", "dhc:denghc@/Register")
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		return err
	}
	stmtIns, err := db.Prepare("INSERT INTO groups (id,group_id,group_member) VALUES (null,?,?)")
	if err != nil {
		return err
	}
	defer stmtIns.Close()

	_, err = stmtIns.Exec(uid, str)
	if err != nil {
		return err
	}
	ct.Unlock()
	log.Debug("Group entity uid: %s added", uid)
	return nil
}

//更新在线表中的群组实体的成员
func (ct *Container) UpdateGroupEntity(uid string, action string, updatelist []string) error {
	switch action {
	case ADD_GROUP_MEMBER:
		ct.Lock()
		if entity, ok := ct.storage[uid]; ok {
			entity.List = append(entity.List, updatelist...)
			str := strings.Join(entity.List, ";")
			db, err := sql.Open("mysql", "dhc:denghc@/Register")
			if err != nil {
				return err
			}
			defer db.Close()

			err = db.Ping()
			if err != nil {
				return err
			}
			stmtIns, err := db.Prepare("UPDATE groups SET group_member = ? where group_id = ?")
			if err != nil {
				return err
			}
			defer stmtIns.Close()

			_, err = stmtIns.Exec(str, uid)
			if err != nil {
				return err
			}
			//ct.Unlock()
			log.Debug("Group entity update: %d added", len(updatelist))
			log.Debug("Group entity update: %d member now", len(entity.List))
		}
		break
	case DEL_GROUP_MEMBER:
		ct.Lock()
		var DeleteFlag int
		if entity, ok := ct.storage[uid]; ok {
			for i := 0; i < len(entity.List); i++ {
				if strings.EqualFold(entity.List[i], updatelist[0]) {
					DeleteFlag = i
				}
			}
			entity.List = append(entity.List[:DeleteFlag], entity.List[DeleteFlag+1:]...)
			str := strings.Join(entity.List, ";")
			db, err := sql.Open("mysql", "dhc:denghc@/Register")
			if err != nil {
				return err
			}
			defer db.Close()

			err = db.Ping()
			if err != nil {
				return err
			}
			stmtIns, err := db.Prepare("UPDATE groups SET group_member = ? where group_id = ?")
			if err != nil {
				return err
			}
			defer stmtIns.Close()

			_, err = stmtIns.Exec(str, uid)
			if err != nil {
				return err
			}
			//ct.Unlock()
			log.Debug("Group entity update: %s delete", updatelist[0])
		}
		break
	}
	ct.Unlock()
	return nil
}

func (ct *Container) GetEntities() error {
	ct.Lock()
	ct.Unlock()
	return nil
}

// 通过UID删除实体
func (ct *Container) DelEntity(uid string) error {
	ct.Lock()
	if _, ok := ct.storage[uid]; ok {
		delete(ct.storage, uid)
		ct.Unlock()
		return nil
	} else {
		ct.Unlock()
		return errors.New("Entity delete failed")
	}
}
