package main

import (
	"context"
	"fmt"
	rejson "github.com/nitishm/go-rejson/v4"
	"github.com/orlangure/gnomock"
	"github.com/redis/go-redis/v9"
	"log"
	"sync"
	"time"
)

func main() {
	// Define a custom Redis preset to use the ReJSON image
	container, err := gnomock.StartCustom("redislabs/rejson:latest", gnomock.DefaultTCP(6379))
	if err != nil {
		log.Fatalf("could not start gnomock container: %v", err)
	}

	defer func() {
		_ = gnomock.Stop(container)
	}()

	// Connect to the Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", container.Host, container.DefaultPort()),
		PoolSize:     32, // goroutine 와 같거나 크게 설정
		MinIdleConns: 32, // 미리 맺어놓을 connection 수, 이 값을 작게 시작하면 첫 performance 테스트가 느리게 측정됨
		MaxIdleConns: 32, // 최대 connection 수 (= 시나리오 중, goroutine을 제일 많이 생성하는 것과 동일한 개수로)
	})

	appCtx := context.Background()

	// Initialize ReJSON handler
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClientWithContext(appCtx, rdb)

	// 데워주자....
	fmt.Println("따뜻해져라...")
	warmWg := sync.WaitGroup{}
	warmWg.Add(1)
	go func() {
		defer warmWg.Done()
		for i := 0; i < 10; i++ {
			_, _ = rh.JSONSet("warmup:100byte", ".", generateLargeJSON(ItemDataSize_100byte))
			_, _ = rh.JSONSet("warmup:500byte", ".", generateLargeJSON(ItemDataSize_500byte))
			_, _ = rh.JSONSet("warmup:1kb", ".", generateLargeJSON(ItemDataSize_1kb))
			_, _ = rh.JSONSet("warmup:10kb", ".", generateLargeJSON(ItemDataSize_10kb))
			_, _ = rh.JSONSet("warmup:100kb", ".", generateLargeJSON(ItemDataSize_100kb))
		}
	}()
	warmWg.Wait()
	time.Sleep(500 * time.Millisecond)
	fmt.Println("따뜻해졌지?")

	// [Note] : Json 으로 취급할 구조체(generate_data.go 의 Item) 는 Field가 30개 있음
	// 아래 fieldDataSize 가 10이면, 30 개의 field 에 총 10byte 짜리 데이터를 넣음

	//////// Measure Set Performance
	// 사이즈 별 비교 (+ 아래 테스트에서 사용할 데이터 미리 만들어놓기)
	fmt.Println()
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - Progress by Size (100 byte to 100kb)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, ItemDataSize_10byte, 1000, 1)
	ScenarioSet(rh, ItemDataSize_100byte, 1000, 1)
	ScenarioSet(rh, ItemDataSize_500byte, 1000, 1)
	ScenarioSet(rh, ItemDataSize_1kb, 1000, 1)
	ScenarioSet(rh, ItemDataSize_10kb, 1000, 1)
	ScenarioSet(rh, ItemDataSize_100kb, 1000, 1)

	// 동시 수행 수별 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - 100 byte + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, ItemDataSize_100byte, 4000, 4)
	ScenarioSet(rh, ItemDataSize_100byte, 8000, 8)
	ScenarioSet(rh, ItemDataSize_100byte, 16000, 16)
	ScenarioSet(rh, ItemDataSize_100byte, 32000, 32)

	// 동시 수행 및 사이즈 별 소곧 비교
	// 100 byte VS 1 kb
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - 1kb  + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, ItemDataSize_1kb, 4000, 4)
	ScenarioSet(rh, ItemDataSize_1kb, 8000, 8)
	ScenarioSet(rh, ItemDataSize_1kb, 16000, 16)
	ScenarioSet(rh, ItemDataSize_1kb, 32000, 32)

	//////////// Measure Get Performance

	// 사이즈 별 비교
	fmt.Println()
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - increase size (100byte to 100kb)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, ItemDataSize_10byte, 1000, 1)
	ScenarioGet(rh, ItemDataSize_100byte, 1000, 1)
	ScenarioGet(rh, ItemDataSize_1kb, 1000, 1)
	ScenarioGet(rh, ItemDataSize_10kb, 1000, 1)
	ScenarioGet(rh, ItemDataSize_100kb, 1000, 1)

	// 동시 수행 에 따른 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - 100 byte + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, ItemDataSize_100byte, 4000, 4)
	ScenarioGet(rh, ItemDataSize_100byte, 8000, 8)
	ScenarioGet(rh, ItemDataSize_100byte, 16000, 16)
	ScenarioGet(rh, ItemDataSize_100byte, 32000, 32)

	// 동시 수행 + 사이즈에 따른 비교
	// 100 byte VS 1kb
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - 1kb + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, ItemDataSize_1kb, 4000, 4)
	ScenarioGet(rh, ItemDataSize_1kb, 8000, 8)
	ScenarioGet(rh, ItemDataSize_1kb, 16000, 16)
	ScenarioGet(rh, ItemDataSize_1kb, 32000, 32)

	//////////// Measure Field Update Performance (사실 SET 명령은 동일하게 사용함)
	fmt.Println()
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - Increase size (100byte to 100kb)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, ItemDataSize_10byte, 1000, 1)
	ScenarioUpdate(rh, ItemDataSize_100byte, 1000, 1)
	ScenarioUpdate(rh, ItemDataSize_500byte, 1000, 1)
	ScenarioUpdate(rh, ItemDataSize_1kb, 1000, 1)
	ScenarioUpdate(rh, ItemDataSize_10kb, 1000, 1)
	ScenarioUpdate(rh, ItemDataSize_100kb, 1000, 1)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - 100 byte + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, ItemDataSize_100byte, 4000, 4)
	ScenarioUpdate(rh, ItemDataSize_100byte, 8000, 8)
	ScenarioUpdate(rh, ItemDataSize_100byte, 16000, 16)
	ScenarioUpdate(rh, ItemDataSize_100byte, 32000, 32)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - 1kb + increase goroutine count (4 to 8 to 16)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, ItemDataSize_1kb, 4000, 4)
	ScenarioUpdate(rh, ItemDataSize_1kb, 8000, 8)
	ScenarioUpdate(rh, ItemDataSize_1kb, 16000, 16)
	ScenarioUpdate(rh, ItemDataSize_1kb, 32000, 32)
}
