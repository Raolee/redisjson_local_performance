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

// 필독!! //
// - 꼭 package 로 main 을 실행해주세요. file 단위로 실행하면 generate_data.go, scenarios.go 를 읽어오지 못해서 작동 못함
func main() {

	// Define a custom Redis preset to use the ReJSON image
	var container *gnomock.Container
	var err error
	usingAlwaysAndFsync := true // 이 변수를 true/false 로 변경해서 테스트 진행
	if usingAlwaysAndFsync {
		container, err = gnomock.StartCustom("redislabs/rejson:latest",
			gnomock.DefaultTCP(6379),
			gnomock.WithHostMounts("c:/dev/redis", "/data"), // [Note] : 본인 PC의 SSD 경로로 지정하세요!
			gnomock.WithCommand("redis-server",
				"--loadmodule", "/usr/lib/redis/modules/rejson.so",
				"--appendonly", "yes",
				"--appendfsync", "always",
			))
		if err != nil {
			log.Fatalf("could not start gnomock container: %v", err)
		}
	} else {
		container, err = gnomock.StartCustom("redislabs/rejson:latest", gnomock.DefaultTCP(6379))
		if err != nil {
			log.Fatalf("could not start gnomock container: %v", err)
		}
	}
	defer func() {
		_ = gnomock.Stop(container)
	}()

	// Connect to the Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", container.Host, container.DefaultPort()),
		PoolSize:     64, // socket connection 을 맺는 최대 수
		MinIdleConns: 16, // 최소 유휴 connection 수
		MaxIdleConns: 32, // 최대 유휴 connection 수
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
			_, _ = rh.JSONSet("warmup:100byte", ".", generateLargeJSON(Maybe100byte))
			_, _ = rh.JSONSet("warmup:500byte", ".", generateLargeJSON(Maybe500byte))
			_, _ = rh.JSONSet("warmup:1kb", ".", generateLargeJSON(Maybe1kb))
			_, _ = rh.JSONSet("warmup:10kb", ".", generateLargeJSON(Maybe10kb))
			_, _ = rh.JSONSet("warmup:100kb", ".", generateLargeJSON(Maybe100kb))
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
	fmt.Println("Measure Set Performance - Progress by Size")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, Maybe10byte, 100, 1)
	ScenarioSet(rh, Maybe100byte, 100, 1)
	ScenarioSet(rh, Maybe500byte, 100, 1)
	ScenarioSet(rh, Maybe1kb, 100, 1)
	ScenarioSet(rh, Maybe10kb, 100, 1)
	ScenarioSet(rh, Maybe100kb, 100, 1)

	// 동시 수행 수별 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - maybe 100 byte + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, Maybe100byte, 4000, 4)
	ScenarioSet(rh, Maybe100byte, 8000, 8)
	ScenarioSet(rh, Maybe100byte, 16000, 16)
	ScenarioSet(rh, Maybe100byte, 32000, 32)

	// 동시 수행 및 사이즈 별 소곧 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - maybe 1kb  + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, Maybe1kb, 4000, 4)
	ScenarioSet(rh, Maybe1kb, 8000, 8)
	ScenarioSet(rh, Maybe1kb, 16000, 16)
	ScenarioSet(rh, Maybe1kb, 32000, 32)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Set Performance - 10kb  + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioSet(rh, Maybe10kb, 4000, 4)
	ScenarioSet(rh, Maybe10kb, 8000, 8)
	ScenarioSet(rh, Maybe10kb, 16000, 16)
	ScenarioSet(rh, Maybe10kb, 32000, 32)

	//////////// Measure Get Performance

	// 사이즈 별 비교
	fmt.Println()
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - process by size")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, Maybe10byte, 100, 1)
	ScenarioGet(rh, Maybe100byte, 100, 1)
	ScenarioGet(rh, Maybe500byte, 100, 1)
	ScenarioGet(rh, Maybe1kb, 100, 1)
	ScenarioGet(rh, Maybe10kb, 100, 1)
	ScenarioGet(rh, Maybe100kb, 100, 1)

	// 동시 수행 에 따른 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - maybe 100 byte + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, Maybe100byte, 4000, 4)
	ScenarioGet(rh, Maybe100byte, 8000, 8)
	ScenarioGet(rh, Maybe100byte, 16000, 16)
	ScenarioGet(rh, Maybe100byte, 32000, 32)

	// 동시 수행 + 사이즈에 따른 비교
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - maybe 1kb + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, Maybe1kb, 4000, 4)
	ScenarioGet(rh, Maybe1kb, 8000, 8)
	ScenarioGet(rh, Maybe1kb, 16000, 16)
	ScenarioGet(rh, Maybe1kb, 32000, 32)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Get Performance - maybe 10kb + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioGet(rh, Maybe10kb, 4000, 4)
	ScenarioGet(rh, Maybe10kb, 8000, 8)
	ScenarioGet(rh, Maybe10kb, 16000, 16)
	ScenarioGet(rh, Maybe10kb, 32000, 32)

	//////////// Measure Field Update Performance (사실 SET 명령은 동일하게 사용함)
	fmt.Println()
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - progress by size")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, Maybe10byte, 100, 1)
	ScenarioUpdate(rh, Maybe100byte, 100, 1)
	ScenarioUpdate(rh, Maybe500byte, 1000, 1)
	ScenarioUpdate(rh, Maybe1kb, 100, 1)
	ScenarioUpdate(rh, Maybe10kb, 100, 1)
	ScenarioUpdate(rh, Maybe100kb, 100, 1)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - maybe 100 byte + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, Maybe100byte, 4000, 4)
	ScenarioUpdate(rh, Maybe100byte, 8000, 8)
	ScenarioUpdate(rh, Maybe100byte, 16000, 16)
	ScenarioUpdate(rh, Maybe100byte, 32000, 32)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - maybe 1kb + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, Maybe1kb, 4000, 4)
	ScenarioUpdate(rh, Maybe1kb, 8000, 8)
	ScenarioUpdate(rh, Maybe1kb, 16000, 16)
	ScenarioUpdate(rh, Maybe1kb, 32000, 32)

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Measure Update Performance (updating one field) - maybe 10kb + increase goroutine count (4 to 32)")
	fmt.Println("-------------------------------------------------------------")
	ScenarioUpdate(rh, Maybe10kb, 4000, 4)
	ScenarioUpdate(rh, Maybe10kb, 8000, 8)
	ScenarioUpdate(rh, Maybe10kb, 16000, 16)
	ScenarioUpdate(rh, Maybe10kb, 32000, 32)
}
