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

// const MapDomainRule = "map-domain-rule"
// const MapIPRule = "map-domain-rule"
const MapStringRule = "map-string-rule"

//var (
//	TakeSpaceValue = []byte{}
//)

//func AddDomainRule(key string, value string) {
//	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
//		//add new to maps
//		log.Debugln("AddDomainRule add new to maps key:%v", key)
//		err := tx.Put(MapDomainRule, []byte(key), []byte(value), 0)
//		if err != nil {
//			log.Errorln("tx.Put(MapDomainRule, []byte(key), []byte(value), 0) %v", err)
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
//	}
//}
//
//func DeleteDomainRule(key string) {
//	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
//		//add new to maps
//		log.Debugln("AddDomainRule add new to maps key:%v", key)
//		err := tx.Delete(MapDomainRule, []byte(key))
//		if err != nil {
//			log.Errorln("tx.Delete(MapDomainRule, []byte(key)) %v", err)
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
//	}
//}
//
//func LoadDomainRule() []C.Rule {
//	rules = []C.Rule{}
//	err := Persistence.RuleDB.View(func(tx *nutsdb.Tx) error {
//		entries, _ := tx.GetAll(MapDomainRule)
//		for _, entry := range entries {
//			key := string(entry.Key)
//			value := string(entry.Value)
//			log.Infoln("LoadDomainRule add R.NewDomainKeyword(key:%v, value:%v)", key, value)
//			rules = append(rules, R.NewDomainKeyword(key, value))
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
//	}
//	return rules
//}
//
//func AddIPRule(key string, value string) {
//	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
//		//add new to maps
//		log.Debugln("AddDomainRule add new to maps key:%v", key)
//		err := tx.Put(MapIPRule, []byte(key), []byte(value), 0)
//		if err != nil {
//			log.Errorln("tx.Put(MapDomainRule, []byte(key), []byte(value), 0) %v", err)
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
//	}
//}
//
//func DeleteIPRule(key string) {
//	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
//		//add new to maps
//		log.Debugln("AddDomainRule add new to maps key:%v", key)
//		err := tx.Delete(MapIPRule, []byte(key))
//		if err != nil {
//			log.Errorln("tx.Delete(MapDomainRule, []byte(key)) %v", err)
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
//	}
//}
//
//func LoadIPRule() []C.Rule {
//	rules = []C.Rule{}
//	err := Persistence.RuleDB.View(func(tx *nutsdb.Tx) error {
//		entries, _ := tx.GetAll(MapIPRule)
//		for _, entry := range entries {
//			key := string(entry.Key)
//			value := string(entry.Value)
//			log.Infoln("LoadIPRule add R.NewIPCIDR(key:%v, value:%v, R.WithIPCIDRNoResolve(true))", key, value)
//			newRule, err := R.NewIPCIDR(key, value, R.WithIPCIDRNoResolve(true))
//			if err != nil {
//				continue
//			}
//			rules = append(rules, newRule)
//		}
//		return nil
//	})
//	if err != nil {
//		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
//	}
//	return rules
//}

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
			log.Errorln("tx.Put(MapDomainRule, []byte(key), []byte(value), 0) %v", err)
		}
		return nil
	})
	if err != nil {
		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
	}
}

func DeleteStrRule(key string) {
	err := Persistence.RuleDB.Update(func(tx *nutsdb.Tx) error {
		//add new to maps
		log.Infoln("DeleteStrRule key:%v", key)
		err := tx.Delete(MapStringRule, []byte(key))
		if err != nil {
			log.Errorln("tx.Delete(MapDomainRule, []byte(key)) %v", err)
		}
		return nil
	})
	if err != nil {
		log.Errorln("AddDomainRule db.Update(func(tx *nutsdb.Tx) error  %v", err)
	}
}

func LoadStrRule() []C.Rule {
	rules = []C.Rule{}
	err := Persistence.RuleDB.View(func(tx *nutsdb.Tx) error {
		entries, _ := tx.GetAll(MapStringRule)
		for _, entry := range entries {
			//key := string(entry.Key)
			value := string(entry.Value)
			strlist := TrimArr(strings.Split(value, ","))
			if len(strlist) != 3 {
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
			rules = append(rules, rule)
		}
		return nil
	})
	if err != nil {
		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
	}
	return rules
}

func LoadRule(rules []C.Rule) []C.Rule {
	newRules := []C.Rule{}
	//newRules = append(newRules, LoadDomainRule()...)
	//newRules = append(newRules, LoadIPRule()...)
	newRules = append(newRules, LoadStrRule()...)
	newRules = append(newRules, rules...)
	return newRules
}
