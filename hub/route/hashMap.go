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

	render.JSON(w, r, render.M{
		"hashMap": items,
	})
}

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

	db_key := req.Type + payload

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
