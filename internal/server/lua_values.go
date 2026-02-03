package server

import (
	"fmt"

	"github.com/yuin/gopher-lua"
)

func goValueToLua(L *lua.LState, value any) lua.LValue {
	if value == nil {
		return lua.LNil
	}
	switch typed := value.(type) {
	case lua.LValue:
		return typed
	case string:
		return lua.LString(typed)
	case []byte:
		return lua.LString(string(typed))
	case bool:
		return lua.LBool(typed)
	case int:
		return lua.LNumber(typed)
	case int64:
		return lua.LNumber(typed)
	case float32:
		return lua.LNumber(typed)
	case float64:
		return lua.LNumber(typed)
	case map[string]any:
		table := L.NewTable()
		for key, item := range typed {
			table.RawSetString(key, goValueToLua(L, item))
		}
		return table
	case map[string]string:
		table := L.NewTable()
		for key, item := range typed {
			table.RawSetString(key, lua.LString(item))
		}
		return table
	case []any:
		table := L.NewTable()
		for i, item := range typed {
			table.RawSetInt(i+1, goValueToLua(L, item))
		}
		return table
	case []string:
		table := L.NewTable()
		for i, item := range typed {
			table.RawSetInt(i+1, lua.LString(item))
		}
		return table
	default:
		return lua.LString(fmt.Sprint(value))
	}
}
