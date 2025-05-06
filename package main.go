package main

import (
	"connectPlcModbus/logger"
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/goburrow/modbus"
	"go.uber.org/zap"
)

// 连接池结构体
type ClientPool struct {
	pool chan modbus.Client
}

func NewClientPool(size int, handler *modbus.TCPClientHandler) *ClientPool {
	p := make(chan modbus.Client, size)
	for i := 0; i < size; i++ {
		if err := handler.Connect(); err != nil {
			logger.Logger.Error("创建连接失败", zap.Error(err))
		}
		p <- modbus.NewClient(handler)
	}
	return &ClientPool{pool: p}
}

func (p *ClientPool) Get() modbus.Client {
	return <-p.pool
}

func (p *ClientPool) Put(client modbus.Client) {
	p.pool <- client
}

func worker(pool *ClientPool, address, quantity uint16, jobs <-chan struct{}, resChan chan []int16, wg *sync.WaitGroup, errorCount *int) {
	defer wg.Done()
	client := pool.Get()
	defer pool.Put(client) // 确保连接归还到池中
	for range jobs {
		results, err := client.ReadHoldingRegisters(address, quantity)
		if err != nil {
			logger.Logger.Error("读取寄存器失败", zap.Error(err))
			*errorCount++
			continue
		}
		// Convert byte slice to int16 slice
		int16Results := make([]int16, len(results)/2)
		for i := range int16Results {
			rawValue := binary.BigEndian.Uint16(results[i*2:])
			int16Results[i] = int16(rawValue)
		}
		logger.Logger.Debug("读取到数据", zap.Int64("时间戳", time.Now().UnixNano()), zap.Any("数据", int16Results))
		resChan <- int16Results
	}
}

func main() {
	// 初始化日志
	logger.InitLogger()
	defer logger.Sync()
	logger.Logger.Debug("应用启动")

	// Modbus TCP 连接参数
	handler := modbus.NewTCPClientHandler("192.168.1.88:502")
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 1

	// 创建连接池
	poolSize := 20 // 连接池大小
	clientPool := NewClientPool(poolSize, handler)
	defer func() {
		// 确保所有连接关闭
		close(clientPool.pool)
		for client := range clientPool.pool {
			if c, ok := client.(interface{ Close() error }); ok {
				c.Close()
			}
		}
	}()

	// 读取参数
	address := uint16(0)    // 寄存器起始地址
	quantity := uint16(125) // 读取数量
	// totalReads := 100_000_000 // 1亿次读取
	totalReads := 1_000 // 总读取次数
	workerCount := 10   // 客户端个数
	errorCount := 0     // 错误计数

	// 准备结果收集
	resChanCount := 1000
	resChan := make(chan []int16, resChanCount) // 缓冲区防止阻塞
	results := make([][]int16, 0, totalReads)
	done := make(chan struct{})

	// 启动结果收集器
	go func() {
		for res := range resChan {
			results = append(results, res)
		}
		close(done)
	}()

	// 创建工作队列
	jobs := make(chan struct{}, totalReads)
	for range totalReads {
		jobs <- struct{}{}
	}
	close(jobs)

	// 记录开始时间
	startTime := time.Now()

	// 启动worker
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go worker(clientPool, address, quantity, jobs, resChan, &wg, &errorCount)
	}

	// 等待所有worker完成
	wg.Wait()
	close(resChan)

	// 等待结果收集完成
	<-done

	// 计算耗时
	duration := time.Since(startTime)

	logger.Logger.Info("性能统计",
		zap.Int("完成读取次数", len(results)),
		zap.Float64("总耗时(秒)", math.Round(duration.Seconds()*100/100)),
		zap.Float64("总耗时(分)", math.Round(duration.Minutes()*100/100)),
		zap.Float64("平均每次耗时(微秒)", math.Round(float64(duration.Microseconds())/float64(totalReads)*100/100)),
		zap.Float64("QPS(次/秒)", math.Round(float64(totalReads)/duration.Seconds()*100/100)),
		zap.Int("错误次数", errorCount),
	)
}

// package main

// import (
// 	"fmt"
// 	"log"
// 	"math"
// 	"time"

// 	"github.com/goburrow/modbus"
// )

// func main() {
// 	// Modbus TCP connection parameters
// 	handler := modbus.NewTCPClientHandler("192.168.1.88:502")
// 	handler.Timeout = 1 * time.Second
// 	handler.SlaveId = 1

// 	// Connect to the PLC
// 	err := handler.Connect()
// 	if err != nil {
// 		log.Fatalf("Failed to connect to PLC: %v", err)
// 	}
// 	defer handler.Close()

// 	client := modbus.NewClient(handler)

// 	// Read holding registers (example: read 10 registers starting from address 0)
// 	address := uint16(0)
// 	quantity := uint16(125)

// 	const Count = 100000000 // Number of times to read the registers

// 	init_time := time.Now().UnixNano()
// 	res := make([][]uint8, 0)
// 	for range Count {
// 		// Read the holding registers from the PLC
// 		results, err := client.ReadHoldingRegisters(address, quantity)
// 		if err != nil {
// 			log.Fatalf("Failed to read holding registers: %v", err)
// 		}
// 		// Print the results
// 		fmt.Printf("Data from PLC:%v %v\n", time.Now().UnixNano(), results)
// 		res = append(res, results)
// 	}

// 	end_time := time.Now().UnixNano()
// 	diff_time := end_time - init_time

// 	fmt.Printf("共耗时: %v秒\n", math.Round(float64(diff_time))/1e9)
// 	fmt.Printf("共连接次数: %v\n", len(res))
// 	fmt.Printf("平均每次耗时: %v微秒\n", math.Round(float64(diff_time)/float64(Count)/1000))
// }
