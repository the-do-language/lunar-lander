# Lunar Lander Lua REST Engine

This project provides a small REST engine built with Gin and GopherLua. Routes are registered from a Lua script using a simple API inspired by Milua's REST approach, but with a lightweight Gin server.

## Quick start

```bash
cat > app.lua <<'LUA'
rest.get("/health", function(req)
  return 200, "ok", { ["content-type"] = "text/plain" }
end)

rest.post("/echo", function(req)
  return 200, { received = req.body, query = req.query }, {}
end)
LUA

go run . -script app.lua -addr :8080
```

## Database (SugarDB)

The engine initializes a lightweight SugarDB store on startup. By default it persists to `sugardb.json` in the working directory. You can configure the location (or disable persistence) with the `-db` flag:

```bash
go run . -script app.lua -addr :8080 -db ./data/sugardb.json
# Or keep everything in memory:
go run . -script app.lua -addr :8080 -db ""
```

### Lua API

SugarDB is exposed to Lua as the `db` module:

```lua
-- Insert a record
local user = db.insert("users", { name = "Ada", role = "pilot" })

-- Query by exact match
local pilots = db.query("users", { role = "pilot" })

-- Update matching records (returns count)
local updated = db.update("users", { name = "Ada" }, { role = "commander" })

-- Delete matching records (returns count)
local deleted = db.delete("users", { role = "commander" })
```

## Lua API

### `rest.get(path, handler)`
### `rest.post(path, handler)`
### `rest.put(path, handler)`
### `rest.patch(path, handler)`
### `rest.delete(path, handler)`
### `rest.any(path, handler)`

Handlers receive a request table:

```lua
function(req)
  -- req.method (string)
  -- req.path (string)
  -- req.raw_query (string)
  -- req.body (string)
  -- req.client_ip (string)
  -- req.headers (table)
  -- req.query (table)
  -- req.params (table)
end
```

Handlers return up to three values:

1. HTTP status code (number, defaults to 200)
2. Body (string or table). Tables are serialized as JSON.
3. Headers table (string keys/values).

Example:

```lua
rest.get("/hello/:name", function(req)
  return 200, { message = "hello " .. req.params.name }, { ["x-powered-by"] = "lua" }
end)
```
