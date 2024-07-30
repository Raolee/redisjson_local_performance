package main

import (
	"encoding/json"
	"fmt"
	"github.com/nitishm/go-rejson/v4"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"
)

func ScenarioSet(rh *rejson.Handler, itemDataSize MaybeItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}

	durations := make([][]time.Duration, concurrentCnt)

	item := generateLargeJSON(itemDataSize)
	for i := 0; i < concurrentCnt; i++ {

		durations[i] = make([]time.Duration, operationCount/concurrentCnt)

		wg.Add(1)
		go func(num int, ops int) {
			defer wg.Done()

			// Measure set performance
			for j := 0; j < ops; j++ {
				startTime := time.Now()
				key := fmt.Sprintf("item:%d:%d:%d", num, j, itemDataSize)
				_, err := rh.JSONSet(key, ".", item)
				if err != nil {
					log.Fatalf("failed to set item in redis: %v", err)
				}
				duration := time.Since(startTime)
				durations[num][j] = duration
			}
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	flattenedDurations := make([]time.Duration, 0, operationCount)
	for _, d := range durations {
		flattenedDurations = append(flattenedDurations, d...)
	}

	totalDuration := time.Since(totalStartTime)
	min, max, avg, p50, p95, p99, p999 := recordStats(flattenedDurations)
	marshalled, _ := json.Marshal(item)

	fmt.Printf("[Total-%d][%d byte] Set %d operations in %v (RPS: %f)\n", concurrentCnt, len(marshalled), operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
	fmt.Printf("\tMin: %v Max: %v Avg: %v\n", min, max, avg)
	fmt.Printf("\tP50: %v P95: %v P99: %v P99.9: %v\n", p50, p95, p99, p999)
}

func ScenarioGet(rh *rejson.Handler, itemDataSize MaybeItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}

	durations := make([][]time.Duration, concurrentCnt)

	for i := 0; i < concurrentCnt; i++ {

		durations[i] = make([]time.Duration, operationCount/concurrentCnt)

		wg.Add(1)
		go func(num, ops int) {
			defer wg.Done()

			for j := 0; j < ops; j++ {
				startTime := time.Now()
				key := fmt.Sprintf("item:%d:%d:%d", num, j, itemDataSize)
				_, err := rh.JSONGet(key, ".")
				if err != nil {
					log.Fatalf("failed to get item from redis: %v", err)
				}
				duration := time.Since(startTime)
				durations[num][j] = duration
			}
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	flattenedDurations := make([]time.Duration, 0, operationCount)
	for _, d := range durations {
		flattenedDurations = append(flattenedDurations, d...)
	}

	totalDuration := time.Since(totalStartTime)
	min, max, avg, p50, p95, p99, p999 := recordStats(flattenedDurations)
	fmt.Printf("[Total-%d][%d byte] Get %d operations in %v (RPS: %f)\n", concurrentCnt, itemDataSize, operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
	fmt.Printf("\tMin: %v Max: %v Avg: %v\n", min, max, avg)
	fmt.Printf("\tP50: %v P95: %v P99: %v P99.9: %v\n", p50, p95, p99, p999)
}

func ScenarioUpdate(rh *rejson.Handler, itemDataSize MaybeItemDataSize, operationCount int, concurrentCnt int) {

	totalStartTime := time.Now()
	wg := sync.WaitGroup{}

	durations := make([][]time.Duration, concurrentCnt)

	newField := generateLargeJSON(itemDataSize).Field1 // 값 하나 결정하고 쭉 Set 함
	for i := 0; i < concurrentCnt; i++ {

		durations[i] = make([]time.Duration, operationCount/concurrentCnt)

		wg.Add(1)
		go func(num, ops int) {
			defer wg.Done()

			for j := 0; j < ops; j++ {
				startTime := time.Now()
				key := fmt.Sprintf("item:%d:%d:%d", num, j, itemDataSize)
				path := fmt.Sprintf(".Field%d", rand.Intn(10)+1) // Random field update
				_, err := rh.JSONSet(key, path, newField)
				if err != nil {
					log.Fatalf("failed to update field in redis: %v", err)
				}
				duration := time.Since(startTime)
				durations[num][j] = duration
			}
		}(i, operationCount/concurrentCnt)
	}
	wg.Wait()

	flattenedDurations := make([]time.Duration, 0, operationCount)
	for _, d := range durations {
		flattenedDurations = append(flattenedDurations, d...)
	}

	totalDuration := time.Since(totalStartTime)
	min, max, avg, p50, p95, p99, p999 := recordStats(flattenedDurations)

	fmt.Printf("[Total-%d][%d byte] Update %d operations in %v (RPS: %f)\n", concurrentCnt, len([]byte(newField)), operationCount, totalDuration, float64(operationCount)/totalDuration.Seconds())
	fmt.Printf("\tMin: %v Max: %v Avg: %v\n", min, max, avg)
	fmt.Printf("\tP50: %v P95: %v P99: %v P99.9: %v\n", p50, p95, p99, p999)
}

func recordStats(durations []time.Duration) (min, max, avg, p50, p95, p99, p999 time.Duration) {
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	count := len(durations)
	min = durations[0]
	max = durations[count-1]
	avg = time.Duration(int64(totalDuration(durations)) / int64(count))
	p50 = durations[count*50/100]
	p95 = durations[count*95/100]
	p99 = durations[count*99/100]
	p999 = durations[count*999/1000]
	return
}

func totalDuration(durations []time.Duration) time.Duration {
	var total time.Duration
	for _, duration := range durations {
		total += duration
	}
	return total
}
