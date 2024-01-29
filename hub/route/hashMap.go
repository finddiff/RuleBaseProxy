package route

import (
	"context"
	"fmt"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	R "github.com/finddiff/RuleBaseProxy/rule"
	"github.com/finddiff/RuleBaseProxy/tunnel"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	//CC "github.com/karlseguin/ccache/v2"
	"net/http"
	"strings"
)

type HashMap struct {
	Type  string `json:"tp"`
	Key   string `json:"key"`
	Value string `json:"proxyName"`
}

func hashMapRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getHashMaps)
	r.Post("/", updateHashMap)

	r.Route("/{key}", func(r chi.Router) {
		r.Use(parseHashMapKey, findHashMapByKey)
		r.Get("/", getHashMap)
	})
	return r
}

func parseHashMapKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := getEscapeParam(r, "key")
		ctx := context.WithValue(r.Context(), CtxKeyHashMapKey, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func findHashMapByKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Context().Value(CtxKeyHashMapKey).(string)
		value, ok := tunnel.Cm.Get(key)
		if ok && value == nil {
			//value, exist := tunnel.Cm.Get(key)
			//if !exist {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, ErrNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), CtxKeyHashMapValue, HashMap{
			Key: key,
			//Value: value.(string),
			Value: value.(string),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getHashMap(w http.ResponseWriter, r *http.Request) {
	hasmap := r.Context().Value(CtxKeyHashMapValue)
	render.JSON(w, r, hasmap)
}

func getHashMaps(w http.ResponseWriter, r *http.Request) {
	items := []HashMap{}
	//for item := range tunnel.Cm.Iter() {
	//	items = append(items, HashMap{
	//		Key:   item.Key,
	//		Value: fmt.Sprintf("%v", item.Val),
	//	})
	//}

	//tunnel.Cm.ForEachFunc(func(key string, i *CC.Item) bool {
	//	items = append(items, HashMap{
	//		Key:   key,
	//		Value: fmt.Sprintf("%v", i.Value()),
	//	})
	//	return true
	//})

	render.JSON(w, r, render.M{
		"hashMap": items,
	})
}

//func clearHashMap() {
//	tunnel.Cm.Clear()
//}

func updateHashMap(w http.ResponseWriter, r *http.Request) {
	req := HashMap{}
	log.Infoln("updateHashMap r.Body:%v", r.Body)
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		log.Errorln("updateHashMap err:%v", ErrBadRequest)
		return
	}

	rule := tunnel.TrimArr(strings.Split(req.Key, ":"))
	target := req.Value
	payload := req.Key
	params := []string{"no-resolve"}
	if len(rule) > 1 {
		payload = rule[0]
		params = rule[1:]
	}

	newRule, err := R.ParseRule(req.Type, payload, target, params)
	if err != nil {
		log.Errorln("ParseRule err:%v", err)
		return
	}

	db_key := req.Type + req.Key

	if req.Value == "DELETE" {
		payload = newRule.Payload()
		var newRule = []C.Rule{}
		for _, rule := range tunnel.Rules() {
			if rule.Payload() != payload {
				newRule = append(newRule, rule)
			} else {
				tunnel.CloseRuleMatchCon(rule)
			}
		}
		tunnel.UpdateRules(newRule)
		tunnel.DeleteStrRule(db_key)
	} else {

		tunnel.UpdateRules(append([]C.Rule{newRule}, tunnel.Rules()...))
		tunnel.CloseRuleMatchCon(newRule)
		tunnel.AddStrRule(db_key, fmt.Sprintf("%s,%s,%s", req.Type, req.Key, req.Value))
	}

	render.NoContent(w, r)
}

//
//func OrgUpdateHashMap(w http.ResponseWriter, r *http.Request) {
//	req := HashMap{}
//	//req :=
//	log.Infoln("updateHashMap r.Body:%v", r.Body)
//	if err := render.DecodeJSON(r.Body, &req); err != nil {
//		render.Status(r, http.StatusBadRequest)
//		render.JSON(w, r, ErrBadRequest)
//		log.Infoln("updateHashMap err:%v", ErrBadRequest)
//		return
//	}
//
//	if strings.Contains(req.Key, ":") {
//		req.Key = req.Key[:strings.Index(req.Key, ":")]
//	}
//
//	//tunnel.Cm.Set(req.Key, req.Value)
//	if req.Value == "DELETE" {
//		var newRule = []C.Rule{}
//		if net.ParseIP(req.Key) != nil {
//			req.Key = req.Key + "/32"
//			tunnel.DeleteIPRule(req.Key)
//		} else {
//			tunnel.DeleteDomainRule(req.Key)
//		}
//		for _, rule := range tunnel.Rules() {
//			if rule.Payload() != req.Key {
//				newRule = append(newRule, rule)
//			} else {
//				tunnel.CloseRuleMatchCon(rule)
//			}
//		}
//		tunnel.UpdateRules(newRule)
//	} else {
//		var newRule C.Rule
//		var err error
//		newRule = nil
//		err = nil
//		if net.ParseIP(req.Key) != nil {
//			newRule, err = R.NewIPCIDR(req.Key+"/32", req.Value, R.WithIPCIDRNoResolve(true))
//			if err == nil {
//				tunnel.UpdateRules(append([]C.Rule{newRule}, tunnel.Rules()...))
//				tunnel.AddIPRule(req.Key+"/32", req.Value)
//			}
//		} else {
//			newRule = R.NewDomainKeyword(req.Key, req.Value)
//			tunnel.UpdateRules(append([]C.Rule{newRule}, tunnel.Rules()...))
//			tunnel.AddDomainRule(req.Key, req.Value)
//		}
//		log.Debugln("CloseRuleMatchCon newRule:%v", newRule)
//		if newRule != nil {
//			tunnel.CloseRuleMatchCon(newRule)
//		}
//	}
//	tunnel.Cm.Clear()
//	log.Infoln("updateHashMap set %v:%v", req.Key, req.Value)
//	render.NoContent(w, r)
//}
