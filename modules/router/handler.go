// 路由模块
package router

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	event_user "github.com/Snipergzf/MessageHive/modules/event/user"
	"github.com/Snipergzf/MessageHive/modules/message"
	"github.com/Snipergzf/MessageHive/modules/onlinetable"
	"github.com/Snipergzf/MessageHive/modules/queue/redis"
	"github.com/Snipergzf/MessageHive/modules/router/transient"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
)

// 消息类型定义
const (
	MESSAGE_TYPE_IDENTITY uint = iota
	MESSAGE_TYPE_AUTHENTICATE
	MESSAGE_TYPE_HEARTBEAT
	MESSAGE_TYPE_RECEIPT
	MESSAGE_TYPE_TRANSIENT
	MESSAGE_TYPE_GROUP
	MESSAGE_TYPE_EVENT
	MESSAGE_INTERN_TYPE_ONLINE
	MESSAGE_INTERN_TYPE_OFFLINE
	MESSAGE_INTERN_TYPE_TRANSIENT
	MESSAGE_TYPE_ERROR = 31
)

// 群组消息类型定义
const (
	MESSAGE_GROUP_JOIN   = "join"
	MESSAGE_GROUP_INVITE = "invite"
	MESSAGE_GROUP_SEND   = "send"
	MESSAGE_GROUP_LEAVE  = "leave"
)

//群成员操作类型定义，用于调用onlinetable.UpdateGroupEntity函数时传递参数
const (
	ADD_GROUP_MEMBER = "add"
	DEL_GROUP_MEMBER = "delete"
)

// 群组消息内容结构
type GroupBody struct {
	Action  string      `json:"action"`
	BodyRaw interface{} `json:"data"`

	List []string
	Data string
}

// 群组消息内容JSON解析
func GroupBodyDecode(r io.Reader) (x *GroupBody, err error) {
	x = new(GroupBody)
	if err = json.NewDecoder(r).Decode(x); err != nil {
		return
	}
	switch t := x.BodyRaw.(type) {
	case string:
		x.Data = t
	case []interface{}:
		bodyraw := x.BodyRaw.([]interface{})
		list := make([]string, 0)
		for i := range bodyraw {
			data, err := bodyraw[i].(string)
			if err == false {
				break
			}
			list = append(list, data)
		}
		x.List = list
	}
	return
}

var log = logging.MustGetLogger("main")

type Config struct {
	mainchan    chan *message.Container
	onlinetable *onlinetable.Container
}

// Returns config of router
func NewConfig(mainchan chan *message.Container, onlinetable *onlinetable.Container) Config {
	return Config{
		mainchan:    mainchan,
		onlinetable: onlinetable,
	}
}

func Handler(config Config) error {
	// Redis init
	redisPool := redis.NewPool(":6379")
	// Event handler init
	eventUserChan := make(chan *event_user.Event, 1000)
	eventUserConfig := event_user.NewConfig(eventUserChan, redisPool, config.mainchan, config.onlinetable)
	go func(config event_user.Config) {
		event_user.Start(eventUserConfig)
	}(eventUserConfig)
	// Transient handler init
	transientChan := make(chan *message.Container, 1000)
	transientConfig := transient.NewConfig(redisPool, transientChan)
	go func(config transient.Config) {
		transient.Handler(config)
	}(transientConfig)

	for {
		select {
		case msg := <-config.mainchan:
			sendflag := true
			sendResponseFlag := true
			sendResToGrpFlag := false
			InviteToGrpFlag := false
			DelFromGrpFlag := false
			sid := msg.GetSID()
			rid := msg.GetRID()
			mtype := msg.GetTYPE()
			sentity, err := config.onlinetable.GetEntity(sid)
			if err != nil {
				// 事件消息不检查发送实体是否存在
				if !hasBit(mtype, MESSAGE_TYPE_EVENT) && !hasBit(mtype, MESSAGE_INTERN_TYPE_OFFLINE) && !hasBit(mtype, MESSAGE_INTERN_TYPE_TRANSIENT) {
					log.Debug(err.Error())
					break
				}
			}
			response := new(message.Container)
			response.MID = proto.String(msg.GetMID())
			response.SID = proto.String("")
			response.RID = proto.String(sid)
			response.TYPE = proto.Uint32(0)
			response.STIME = proto.Int64(time.Now().Unix())
			response.BODY = proto.String("")
			for i := 0; i <= int(MESSAGE_TYPE_ERROR); i++ {
				// 消息分类处理
				if hasBit(mtype, uint(i)) {
					switch uint(i) {
					case MESSAGE_INTERN_TYPE_ONLINE:
						e := &event_user.Event{
							Uid:  sid,
							Type: event_user.USER_ONLINE,
						}
						eventUserChan <- e
						sendflag = false
						break
					case MESSAGE_INTERN_TYPE_OFFLINE:
						e := &event_user.Event{
							Uid:  sid,
							Type: event_user.USER_OFFLINE,
						}
						eventUserChan <- e
						sendflag = false
						sendResponseFlag = false
						break
					case MESSAGE_TYPE_EVENT:
						sendResponseFlag = false
						break
					case MESSAGE_TYPE_GROUP:
						body := msg.GetBODY()
						groupbody, err := GroupBodyDecode(strings.NewReader(body))
						if err != nil {
							log.Info(err.Error())
						}
						switch groupbody.Action {
						case MESSAGE_GROUP_JOIN:
							_, err := config.onlinetable.GetEntity(rid)
							if err != nil {
								err = config.onlinetable.AddGroupEntity(rid, groupbody.List, true)
								if err != nil {
									log.Error(err.Error())
								}
							}
							sendflag = false
							sendResToGrpFlag = true
							// TODO
							break
						case MESSAGE_GROUP_SEND:
							// PASS
							break
						case MESSAGE_GROUP_INVITE:
							// TODO
							err := config.onlinetable.UpdateGroupEntity(rid, ADD_GROUP_MEMBER, groupbody.List)
							if err != nil {
								log.Error(err.Error())
							}
							sendflag = false
							InviteToGrpFlag = true
							break
						//删除群组成员
						case MESSAGE_GROUP_LEAVE:
							// TODO
							err := config.onlinetable.UpdateGroupEntity(rid, DEL_GROUP_MEMBER, groupbody.List)
							if err != nil {
								log.Error(err.Error())
							}
							sendflag = false
							DelFromGrpFlag = true
							break
						}
						break
					}
				}
			}
			// 发送回应消息
			go func(flag1 bool, falg2 bool, flag3 bool, falg4 bool) {
				//若为群组消息，改response的BODY
				if sendResToGrpFlag {
					response.BODY = proto.String(`{"action":"join","data":"succeed"}`)
				}
				if InviteToGrpFlag {
					response.BODY = proto.String(`{"action":"invite","data":"succeed"}`)
				}
				//删除群组成员
				if DelFromGrpFlag {
					response.BODY = proto.String(`{"action":"leave","data":"succeed"}`)
				}
				if sendResponseFlag {
					select {
					case sentity.Pipe <- response:
						log.Info("Response delivered to %s", sid)
					case <-time.After(time.Second):
						log.Error("Response failed to deliverd to %s", sid)
					}
				}
			}(sendResToGrpFlag, InviteToGrpFlag, sendResponseFlag, DelFromGrpFlag)

			// Send to rid
			if sendflag {
				rentity, err := config.onlinetable.GetEntity(rid)
				if err != nil {
					log.Debug(err.Error())
					if hasBit(mtype, MESSAGE_TYPE_TRANSIENT) {
						// MESSAGE_TYPE_TRANSIENT
						// 非暂态的消息向Transient队列压入消息
						msg.TYPE = proto.Uint32(setBit(mtype, MESSAGE_INTERN_TYPE_TRANSIENT))
						transientChan <- msg
					}
					break
				}
				switch rentity.Type {
				case onlinetable.ENTITY_TYPE_GROUP:
					for _, v := range rentity.List {
						if v != sid {
							newmsg := *msg
							newmsg.RID = proto.String(v)
							config.mainchan <- &newmsg
						}
					}
				case onlinetable.ENTITY_TYPE_USER:
					go func() {
						select {
						case rentity.Pipe <- msg: // TODO: 这里可能造成死锁
							log.Info("Message delivered from %s to %s", sid, rid)
						case <-time.After(time.Second):
							config.mainchan <- msg
						}
					}()
				}
			}
		}
	}
	return nil
}

// Assume BigEndian

func hasBit(n uint32, pos uint) bool {
	val := n & (1 << pos)
	return (val > 0)
}

func setBit(n uint32, pos uint) uint32 {
	n |= (1 << pos)
	return n
}
