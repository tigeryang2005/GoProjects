package main

import (
	"connectPlcModbus/inovanceModbus"
	"connectPlcModbus/logger"
)

func main() {
	// 初始化日志
	logger.InitLogger()
	defer logger.Sync()
	logger.Logger.Debug("应用启动")

	inovanceModbus.ConnectInovanceModbus()

}
