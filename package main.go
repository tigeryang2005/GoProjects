package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/goburrow/modbus"
)

func readRegisters(client modbus.Client, address, quantity uint16, resChan chan []int16, errorCount *int) {
	results, err := client.ReadHoldingRegisters(address, quantity)
	if err != nil {
		log.Printf("Failed to read holding registers: %v", err)
		*errorCount++
		return
	}
	// Convert byte slice to int16 slice
	int16Results := make([]int16, len(results)/2)
	for i := range int16Results {
		rawValue := binary.BigEndian.Uint16(results[i*2:])
		int16Results[i] = int16(rawValue)
	}
	fmt.Printf("读取到数据:%v %v\n", time.Now().UnixNano(), int16Results)
	resChan <- int16Results
}

func worker(client modbus.Client, address, quantity uint16, jobs <-chan struct{}, resChan chan []int16, wg *sync.WaitGroup, errorCount *int) {
	defer wg.Done()
	for range jobs {
		readRegisters(client, address, quantity, resChan, errorCount)
	}
}

func main() {
	// Modbus TCP 连接参数
	handler := modbus.NewTCPClientHandler("192.168.1.88:502")
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 1

	// 连接到 PLC
	err := handler.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to PLC: %v", err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)

	// 读取参数
	address := uint16(0)
	quantity := uint16(125)
	// totalReads := 100_000_000 // 1亿次读取
	totalReads := 10_000 // 总读取次数
	workerCount := 30    // goroutine线程个数
	errorCount := 0      // 错误计数

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
		go worker(client, address, quantity, jobs, resChan, &wg, &errorCount)
	}

	// 等待所有worker完成
	wg.Wait()
	close(resChan)

	// 等待结果收集完成
	<-done

	// 计算耗时
	duration := time.Since(startTime)

	fmt.Printf("总共完成读取: %d 次\n", len(results))
	fmt.Printf("总耗时: %.2f 秒\n", duration.Seconds())
	fmt.Printf("总耗时: %.2f 分\n", float64(duration.Minutes()))
	fmt.Printf("平均每次耗时: %.2f 微秒\n", float64(duration.Microseconds())/float64(totalReads))
	fmt.Printf("QPS: %.0f 次/秒\n", float64(totalReads)/duration.Seconds())
	fmt.Printf("错误次数: %d\n", errorCount)
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
