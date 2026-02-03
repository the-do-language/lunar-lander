package server

import (
	"github.com/gin-gonic/gin"

	"lunar-lander/internal/sugardb"
)

type Runtime struct {
	Router *gin.Engine
	Engine *LuaEngine
}

func BuildRuntime(scriptPath string, store *sugardb.Store) (*Runtime, error) {
	router := gin.New()
	router.Use(gin.Recovery())

	engine := NewLuaEngine(router, store)
	if err := engine.LoadScript(scriptPath); err != nil {
		engine.Close()
		return nil, err
	}

	return &Runtime{
		Router: router,
		Engine: engine,
	}, nil
}
