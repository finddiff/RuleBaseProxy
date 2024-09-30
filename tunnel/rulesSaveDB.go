package tunnel

import (
	"github.com/finddiff/RuleBaseProxy/Persistence"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	R "github.com/finddiff/RuleBaseProxy/rule"
	//"github.com/xujiajun/nutsdb"
	nutsdb "github.com/finddiff/nutsDBMD"
	"strings"
)

const MapStringRule = "map-string-rule"

func TrimArr(arr []string) (r []string) {
	for _, e := range arr {
		r = append(r, strings.Trim(e, " "))
	}
	return
}

func AddStrRule(key string, value string) {
	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
		//add new to maps
		log.Debugln("AddStrRule add new to maps key:%v value:%v", key, value)
		err := tx.Put(MapStringRule, []byte(key), []byte(value), 0)
		if err != nil {
			log.Errorln("tx.Put(MapStringRule, []byte(key), []byte(value), 0) %v", err)
		}
		return nil
	})
	if err != nil {
		log.Errorln("AddStrRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
	}
}

func DeleteStrRule(key string) {
	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
		//add new to maps
		log.Infoln("DeleteStrRule key:%v", key)
		err := tx.Delete(MapStringRule, []byte(key))
		if err != nil {
			log.Errorln("tx.Delete(MapStringRule, []byte(key)) %v", err)
		}
		return nil
	})
	if err != nil {
		log.Errorln("DeleteStrRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
	}
}

func LoadStrRule() []C.Rule {
	rules = []C.Rule{}
	needReSetKey := map[string]string{}
	deleteKey := []string{}
	err := Persistence.RuleDB.View(func(tx *nutsdb.Tx) error {
		entries, _ := tx.GetAll(MapStringRule)
		for _, entry := range entries {
			key := string(entry.Key)
			value := string(entry.Value)
			strlist := TrimArr(strings.Split(value, ","))
			if len(strlist) != 3 {
				deleteKey = append(deleteKey, key)
				log.Debugln("LoadStrRule err %s len(strlist) != 3", value)
				continue
			}
			param := []string{"no-resolve"}
			payload := strlist[1]
			if strings.Count(strlist[1], ":") > 0 {
				rule := TrimArr(strings.Split(payload, ":"))
				payload = rule[0]
				param = rule[1:]
			}
			rule, parseErr := R.ParseRule(strlist[0], payload, strlist[2], param)
			if parseErr != nil {
				log.Debugln("LoadStrRule parseErr:%v", parseErr)
				continue
			}
			log.Infoln("LoadStrRule add rule：%v", rule)
			if key != rule.RuleType().String()+rule.Payload() {
				log.Infoln("LoadStrRule key not match key:%s, value:%s", key, value)
				deleteKey = append(deleteKey, key)
				needReSetKey[rule.RuleType().String()+rule.Payload()] = value
			}
			rules = append(rules, rule)
		}
		return nil
	})
	//出现错误key 的时候进行修复，统一使用 rule.RuleType().String()+rule.Payload() 做为key；重新存入数据库
	for key, value := range needReSetKey {
		AddStrRule(key, value)
	}
	//删除错误的key
	for _, key := range deleteKey {
		DeleteStrRule(key)
	}
	if err != nil {
		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
	}
	return rules
}

func LoadRule(rules []C.Rule) []C.Rule {
	newRules := []C.Rule{}
	newRules = append(newRules, LoadStrRule()...)
	newRules = append(newRules, rules...)
	return newRules
}
