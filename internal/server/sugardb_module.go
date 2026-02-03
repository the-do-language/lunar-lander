package server

import (
	"sort"

	"github.com/yuin/gopher-lua"

	"lunar-lander/internal/sugardb"
)

func registerSugarDBModule(L *lua.LState, store *sugardb.Store) {
	if store == nil {
		store = sugardb.NewStore()
	}
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get":    sugarGet(store),
		"set":    sugarSet(store),
		"delete": sugarDelete(store),
		"keys":   sugarKeys(store),
	})
	L.SetGlobal("sugardb", mod)
}

func sugarGet(store *sugardb.Store) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		value, ok := store.Get(key)
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(goValueToLua(L, value))
		return 1
	}
}

func sugarSet(store *sugardb.Store) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.CheckAny(2)
		store.Set(key, luaValueToGo(value))
		L.Push(lua.LTrue)
		return 1
	}
}

func sugarDelete(store *sugardb.Store) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		L.Push(lua.LBool(store.Delete(key)))
		return 1
	}
}

func sugarKeys(store *sugardb.Store) lua.LGFunction {
	return func(L *lua.LState) int {
		keys := store.Keys()
		sort.Strings(keys)
		table := L.NewTable()
		for i, key := range keys {
			table.RawSetInt(i+1, lua.LString(key))
		}
		L.Push(table)
		return 1
	}
}
