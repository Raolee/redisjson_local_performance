package main

import (
	"fmt"
	"github.com/nitishm/go-rejson/v4"
	"log"
	"math/rand"
	"sync"
	"time"
)

func ScenarioSet(rh *rejson.Handler, itemDataSize ItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentCnt; i++ {
		wg.Add(1)
		go func(num int, ops int) {
			defer wg.Done()
			item := generateLargeJSON(itemDataSize)

			// Set up performance test
			startTime := time.Now()

			// Measure set performance
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("item:%d:%d", i, itemDataSize)
				_, err := rh.JSONSet(key, ".", item)
				if err != nil {
					log.Fatalf("failed to set item in redis: %v", err)
				}
			}
			duration := time.Since(startTime)
			setRPS := float64(ops) / duration.Seconds()
			_ = setRPS
			// [Note] : 디테일을 보고 싶으면 아래 주석 제거
			// fmt.Printf("[%d][%d byte] Set %d operations in %v (RPS: %f)\n", num, itemDataSize, ops, duration, setRPS)
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	totalDuration := time.Since(totalStartTime)

	fmt.Printf("[Total-%d][%d byte] Set %d operations in %v (RPS: %f)\n", concurrentCnt, itemDataSize, operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
}

func ScenarioGet(rh *rejson.Handler, itemDataSize ItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentCnt; i++ {
		wg.Add(1)
		go func(num, ops int) {
			defer wg.Done()
			startTime := time.Now()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("item:%d:%d", i, itemDataSize)
				_, err := rh.JSONGet(key, ".")
				if err != nil {
					log.Fatalf("failed to get item from redis: %v", err)
				}
			}
			duration := time.Since(startTime)
			getRPS := float64(ops) / duration.Seconds()
			_ = getRPS
			// [Note] : 디테일을 보고 싶으면 아래 주석 제거
			//fmt.Printf("[%d][%d byte] Get %d operations in %v (RPS: %f)\n", num, itemDataSize, ops, duration, getRPS)
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	totalDuration := time.Since(totalStartTime)
	fmt.Printf("[Total-%d][%d byte] Get %d operations in %v (RPS: %f)\n", concurrentCnt, itemDataSize, operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
}

func ScenarioUpdate(rh *rejson.Handler, itemDataSize ItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentCnt; i++ {
		wg.Add(1)
		go func(num, ops int) {
			defer wg.Done()
			// Measure field update performance
			newField := generateLargeJSON(itemDataSize).Field1 // 값 하나 결정하고 쭉 Set 함
			startTime := time.Now()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("item:%d:%d", i, itemDataSize)
				path := fmt.Sprintf(".Field%d", rand.Intn(30)+1) // Random field update
				_, err := rh.JSONSet(key, path, newField)
				if err != nil {
					log.Fatalf("failed to update field in redis: %v", err)
				}
			}
			duration := time.Since(startTime)
			updateRPS := float64(ops) / duration.Seconds()
			_ = updateRPS
			// [Note] : 디테일을 보고 싶으면 아래 주석 제거
			//fmt.Printf("[%d][%d byte] Update %d operations in %v (RPS: %f)\n", num, itemDataSize, ops, duration, updateRPS)
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	totalDuration := time.Since(totalStartTime)
	fmt.Printf("[Total-%d][%d byte] Update %d operations in %v (RPS: %f)\n", concurrentCnt, itemDataSize, operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
}
