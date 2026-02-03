package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/yuin/gopher-lua"

	"lunar-lander/internal/sugardb"
)

type LuaEngine struct {
	L     *lua.LState
	mu    sync.Mutex
	route *gin.Engine
	refs  []*lua.LFunction
}

func NewLuaEngine(router *gin.Engine, store *sugardb.Store) *LuaEngine {
	L := lua.NewState()
	engine := &LuaEngine{L: L, route: router}
	registerRestModule(L, engine)
	registerSugarDBModule(L, store)
	return engine
}

func (e *LuaEngine) Close() {
	if e == nil || e.L == nil {
		return
	}
	e.L.Close()
}

func (e *LuaEngine) LoadScript(path string) error {
	if path == "" {
		return errors.New("script path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return e.L.DoFile(path)
}

func registerRestModule(L *lua.LState, engine *LuaEngine) {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get":    engine.registerRoute(http.MethodGet),
		"post":   engine.registerRoute(http.MethodPost),
		"put":    engine.registerRoute(http.MethodPut),
		"patch":  engine.registerRoute(http.MethodPatch),
		"delete": engine.registerRoute(http.MethodDelete),
		"any":    engine.registerAnyRoute,
	})
	L.SetGlobal("rest", mod)
}

func (e *LuaEngine) registerAnyRoute(L *lua.LState) int {
	path := L.CheckString(1)
	handler := L.CheckFunction(2)
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}
	for _, method := range methods {
		e.attachRoute(method, path, handler)
	}
	return 0
}

func (e *LuaEngine) registerRoute(method string) lua.LGFunction {
	return func(L *lua.LState) int {
		path := L.CheckString(1)
		handler := L.CheckFunction(2)
		e.attachRoute(method, path, handler)
		return 0
	}
}

func (e *LuaEngine) attachRoute(method, path string, handler *lua.LFunction) {
	e.refs = append(e.refs, handler)
	e.route.Handle(method, path, func(c *gin.Context) {
		e.mu.Lock()
		defer e.mu.Unlock()

		reqTable, err := requestToLuaTable(e.L, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := e.L.CallByParam(lua.P{Fn: handler, NRet: 3, Protect: true}, reqTable); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		status, body, headers := parseLuaResponse(e.L)
		for k, v := range headers {
			c.Header(k, v)
		}
		if body == nil {
			c.Status(status)
			return
		}
		switch typed := body.(type) {
		case string:
			c.String(status, typed)
		case []byte:
			c.Data(status, "application/octet-stream", typed)
		default:
			c.JSON(status, typed)
		}
	})
}

func requestToLuaTable(L *lua.LState, c *gin.Context) (*lua.LTable, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	req := L.NewTable()
	req.RawSetString("method", lua.LString(c.Request.Method))
	req.RawSetString("path", lua.LString(c.Request.URL.Path))
	req.RawSetString("raw_query", lua.LString(c.Request.URL.RawQuery))
	req.RawSetString("body", lua.LString(string(body)))
	req.RawSetString("client_ip", lua.LString(c.ClientIP()))

	headers := L.NewTable()
	headerKeys := make([]string, 0, len(c.Request.Header))
	for k := range c.Request.Header {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)
	for _, k := range headerKeys {
		values := c.Request.Header[k]
		if len(values) == 1 {
			headers.RawSetString(k, lua.LString(values[0]))
			continue
		}
		list := L.NewTable()
		for i, v := range values {
			list.RawSetInt(i+1, lua.LString(v))
		}
		headers.RawSetString(k, list)
	}
	req.RawSetString("headers", headers)

	query := L.NewTable()
	for k, values := range c.Request.URL.Query() {
		if len(values) == 1 {
			query.RawSetString(k, lua.LString(values[0]))
			continue
		}
		list := L.NewTable()
		for i, v := range values {
			list.RawSetInt(i+1, lua.LString(v))
		}
		query.RawSetString(k, list)
	}
	req.RawSetString("query", query)

	params := L.NewTable()
	for _, p := range c.Params {
		params.RawSetString(p.Key, lua.LString(p.Value))
	}
	req.RawSetString("params", params)

	return req, nil
}

func parseLuaResponse(L *lua.LState) (int, any, map[string]string) {
	defer L.Pop(3)
	statusValue := L.Get(-3)
	bodyValue := L.Get(-2)
	headersValue := L.Get(-1)

	status := http.StatusOK
	if statusValue.Type() == lua.LTNumber {
		status = int(lua.LVAsNumber(statusValue))
	}

	var body any
	switch v := bodyValue.(type) {
	case lua.LString:
		body = string(v)
	case *lua.LTable:
		body = luaTableToMap(v)
	case lua.LBool:
		body = bool(v)
	case lua.LNumber:
		body = float64(v)
	case *lua.LNilType:
		body = nil
	default:
		body = v.String()
	}

	headers := map[string]string{}
	if table, ok := headersValue.(*lua.LTable); ok {
		table.ForEach(func(key, value lua.LValue) {
			if key.Type() == lua.LTString && value.Type() == lua.LTString {
				headers[key.String()] = value.String()
			}
		})
	}

	return status, body, headers
}

func luaTableToMap(table *lua.LTable) map[string]any {
	result := map[string]any{}
	table.ForEach(func(key, value lua.LValue) {
		result[key.String()] = luaValueToGo(value)
	})
	return result
}

func luaValueToGo(value lua.LValue) any {
	switch v := value.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		return luaTableToMap(v)
	default:
		return v.String()
	}
}
