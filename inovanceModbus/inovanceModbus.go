package inovanceModbus

import (
	"connectPlcModbus/logger"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goburrow/modbus"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"go.uber.org/zap"
)

func ConnectInovanceModbus() {
	//连接influxdb
	// 1. 创建InfluxDB客户端
	url := "http://localhost:8086"
	token := "Q52Nrc8xXl1gWdYfHYmUkmdxLft0FnXR7gxuUKxMjC8YBo6ra8m1G1ff3EFufC42lMea6qI4eDmqovlEc7-XUA=="
	org := "my-org"
	bucket := "my-bucket"
	influxdb2Client := influxdb2.NewClient(url, token)
	defer influxdb2Client.Close()
	// 获取异步写入API
	writeAPI := influxdb2Client.WriteAPI(org, bucket)
	// 检查数据库连接健康
	health, err := influxdb2Client.Health(context.Background())
	if err != nil {
		logger.Logger.Error("健康检查失败:", zap.Error(err))
	}
	logger.Logger.Info("InfluxDB 健康状态", zap.Any("status:", health.Status))
	modbusClientSize := 1 // 客户端handler数量与worker数量一致
	modbusClients := make([]modbus.Client, 0, modbusClientSize)
	for range modbusClientSize {
		// Modbus TCP 连接参数
		handler := modbus.NewTCPClientHandler("192.168.1.88:502")
		handler.Timeout = 1 * time.Second
		handler.SlaveId = 1
		if err := handler.Connect(); err != nil {
			logger.Logger.Error("创建连接失败", zap.Error(err))
		}
		modbusClient := modbus.NewClient(handler)
		modbusClients = append(modbusClients, modbusClient)
	}

	// 读取参数
	address := uint16(0)    // 寄存器起始地址
	quantity := uint16(125) // 读取数量

	// totalReads := 100_000_000 // 1亿次读取
	totaljobs := int64(100_000) // 总读取次数 0表示无限读取
	currentJobs := int64(0)     // 已读取次数计数
	errorCount := int32(0)      // 错误计数

	// 准备结果收集
	resChanCount := 1000
	resChan := make(chan map[time.Time][]int16, resChanCount) // 缓冲区防止阻塞
	done := make(chan struct{})

	// 记录开始时间
	startTime := time.Now()

	// 启动结果收集器
	go func(writeAPI api.WriteAPI, startTime time.Time, currentJobs *int64) {
		defer close(done)
		for res := range resChan {
			// results = append(results, res)
			for key, value := range res {
				// 创建point
				p := influxdb2.NewPointWithMeasurement("experiment").AddTag("location", "tianjin")
				for count, v := range value {
					p.AddField(fmt.Sprintf("sensor%d", count), v)
				}
				// todo:后面要把观察点删除，把入库放到worker里
				atomic.AddInt64(currentJobs, 1)
				p.AddField("num", *currentJobs)
				// 写入数据到InfluxDB
				p.SetTime(time.Unix(0, key.UnixNano()))
				// logger.Logger.Info("写入数据", zap.Int64("时间戳", key.UnixNano()), zap.Any("num:", *currentJobs), zap.Any("值：", value))
				writeAPI.WritePoint(p)
				// 输出统计信息
				// logger.Logger.Debug("写入完成", zap.Int64("总读取次数", *currentJobs))
				if *currentJobs%10_000 == 0 {
					resInfo(startTime, errorCount, *currentJobs)
				}
			}
		}
		// 确保所有数据都已写入
		writeAPI.Flush()

		// 错误处理
		errorsCh := writeAPI.Errors()
		go func() {
			for err := range errorsCh {
				logger.Logger.Error("写入错误:", zap.Error(err))
			}
		}()
	}(writeAPI, startTime, &currentJobs)

	jobs := make(chan struct{}, 1000)
	interval := 1 * time.Microsecond // 读取间隔1微秒
	// 启动周期性任务生成器
	go continuousJobGenerator(interval, jobs, totaljobs)

	// 启动worker
	var wg sync.WaitGroup
	for _, modbusClient := range modbusClients {
		wg.Add(1)
		go worker(modbusClient, address, quantity, jobs, resChan, &wg, &errorCount)
	}

	// 等待所有worker完成
	wg.Wait()
	close(resChan)

	// 等待结果收集完成
	<-done

}

func worker(modbusClient modbus.Client, address, quantity uint16, jobs <-chan struct{}, resChan chan map[time.Time][]int16, wg *sync.WaitGroup, errorCount *int32) {
	defer wg.Done()
	for range jobs {
		readRegistersTs := time.Now()
		results, err := modbusClient.ReadHoldingRegisters(address, quantity)
		if err != nil {
			logger.Logger.Error("读取寄存器失败", zap.Error(err))
			atomic.AddInt32(errorCount, 1)
			continue
		}
		// Convert byte slice to int16 slice
		int16Results := make([]int16, len(results)/2)
		for i := range int16Results {
			rawValue := binary.BigEndian.Uint16(results[i*2:])
			int16Results[i] = int16(rawValue)
		}
		resultsMap := make(map[time.Time][]int16)
		resultsMap[readRegistersTs] = int16Results
		// 记录读取到的数据
		// logger.Logger.Debug("读取到数据", zap.Int64("时间戳", readRegistersTs.UnixNano()), zap.Any("数据", int16Results))
		resChan <- resultsMap
	}
}

// 使用 time.Ticker 实现周期性任务和定长任务生成
func continuousJobGenerator(interval time.Duration, jobs chan<- struct{}, totalJobs int64) {

	// 如果 totalJobs 为 0，表示无限读取
	// 否则，表示定长读取
	if totalJobs == int64(0) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			jobs <- struct{}{}
		}
	} else {
		for range totalJobs {
			jobs <- struct{}{}
		}
	}
	defer close(jobs)
}

func resInfo(startTime time.Time, errorCount int32, currentJobs int64) {
	duration := time.Since(startTime)
	logger.Logger.Info("性能统计",
		zap.Int64("完成读取次数", currentJobs),
		zap.Float64("总耗时(秒)", math.Round(duration.Seconds()*100/100)),
		zap.Float64("总耗时(分)", math.Round(duration.Minutes()*100/100)),
		zap.Float64("平均每次耗时(微秒)", math.Round(float64(duration.Microseconds())/float64(currentJobs)*100/100)),
		zap.Float64("QPS(次/秒)", math.Round(float64(currentJobs)/duration.Seconds()*100/100)),
		zap.Int32("错误次数", errorCount),
	)
}
