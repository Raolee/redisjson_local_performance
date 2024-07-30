# Redis 성능 테스트 with ReJSON

JSON 데이터를 처리하기 위한 ReJSON 모듈을 사용하여 Redis의 기본적인 성능을 검증해보는 프로젝트.  
테스트는 Redis에서 JSON 객체에 대한 CRU 를 수행함. (json size 및 동시수행할 goroutine 을 변화해가면서 등)
### 사전 요구 사항
- Go 1.22
- Docker
- Git

### 설정
- 없지요.

### 의존성 설치
```bash
go mod tidy
```
----
### 요약: 본 프로그램을 돌리면서 얻은 확신들 정리
1. 내 pc는 스타2, 오버워치도 잘 돌아가지 않는 낡은 11살짜리 노트북이다.
2. 로컬로 실행했을 때, **`1kb 사이즈 JSON도 6000 RPS 이상의 처리를 해낸다. 이 정도면 꽤나 다양한 비즈니스 요구사항에서 쓸만한 수치다.`**
3. 클라우드 환경에서 Standalone으로 써도 머신 성능이 좋다면 더 많은 처리를 할 수 있을 것 같다. (왜냐고? Redis에서 JSON 직렬/역직렬하는 것은 멀티코어를 쓰기 때문)
4. AWS MemoryDB for redis는 cluster 모드가 되며, 아래 RedisJSON 을 지원하므로 **`더 많은 양의 쓰기를 빠르게 처리하고자 한다면, 수만~수십만 RPS를 뽑을 수 있을 것`**
5. 특정 Field를 업데이트하는 것은 없다. 결국 **`SET 을 동일하게 이용하되, path를 지정해서 `단 한 개의 Field를 SET 하는 것` 으로 처리된다.`**
----

## 본 프로젝트의 실험 시작
`cmd/main.go` 의 코드는 `gnomock을` 사용하여 Redis 및 ReJSON이 사전 설치된 Docker 컨테이너를 시작합니다.  
그리고 코드로 정의된 시나리오를 실행하며, stdout 으로 결과를 출력합니다.  
(로컬에서 Docker 실행 중이어야 합니다.)

### 테스트 실행

cmd/main.go 를 실행합니다.  
(file 단위 실행이 아닌, package 단위로 실행 필요) 

#### 1. Redis Client 연결
본 테스트에서 중요한 Client option은 아래 3개입니다. `PoolSize`, `MinIdleConns`, `MaxIdleConns`

- **PoolSize**: 예상되는 동시 요청을 관리하는 Pool 크기
- **MinIdleConns**: 초기에 연결한 Redis connection 수
- **MaxIdleConns**: 최대 연결할 수 있는 Redis connection 수

##### 위 옵션과의 성능 시나리와의 관계
1. 실제 PoolSize 가 작다면, 아무리 요청을 보내도 wating time이 길어져서 성능 측정 결과가 좋지 않게 나옵니다.
2. MinIdleConns 가 작은 상태에서 많은 요청을 받으면, MaxIdleConns 까지 커넥션을 늘리게되는데 이 시간이 집계에 포함되어 느리게 측정됩니다.
3. MaxIdleConns 이 작다면, goroutine 으로 많은 동시 요청을 보내도, 커넥션을 획득하기 전까지 대기 하므로 정확한 퍼포먼스 측정이 어렵습니다.

```go
// Connect to the Redis server
rdb := redis.NewClient(&redis.Options{
    Addr:         fmt.Sprintf("%s:%d", container.Host, container.DefaultPort()),
    PoolSize:     32, // goroutine 와 같거나 크게 설정
    MinIdleConns: 32, // 미리 맺어놓을 connection 수, 이 값을 작게 시작하면 첫 performance 테스트가 느리게 측정됨
    MaxIdleConns: 32, // 최대 connection 수 (= 시나리오 중, goroutine을 제일 많이 생성하는 것과 동일한 개수로)
})

```

#### 2. 워밍업 단계
주요 성능 테스트를 시작하기 전에, Redis 연결과 캐시를 초기화하기 위해 워밍업 단계를 수행합니다.  
`아래 단계를 수행하지 않으면(=주석처리) 하면 실제 성능테스트의 첫 시나리오의 퍼포먼스가 조금 더 나쁘게 측정`됩니다.  
첫 번째 시나리오를 수행할 때, 약간의 성능 저하가 발생하는 것은 일반적인 경우이긴 합니다.  

정밀한 원인 분석이 필요하다면 go의 `pprof` 를 이용해서 분석을 하면되는데,  
**본 프로젝트는 RedisJSON 의 퍼포먼스를 로컬 단위에서 측정하는 것이 목표**이므로 생략하고, 단순한 Wamr-up을 수행 합니다.
```go
// 워밍업 단계
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
```

#### 3. 성능 시나리오
##### 설정 성능 (Set Performance)
1. 데이터 크기별: SET 하는 대상의 크기를 10바이트에서 100KB로 증가시키며 1천회 수행
2. 동시성별 (100 byte, 1kb): SET 을 수행하는 고루틴 수를 4 to 8 to 16 to 32로 증가하여 수행
##### 가져오기 성능 (Get Performance)
1. 데이터 크기별: SET 성능 테스트와 유사하지만, Redis 에서 데이터를 읽는 작업을 수행합니다.
2. 동시성별 (100byte, 1kb): 데이터를 다양한 동시성 수준으로 읽는 작업을 수행합니다.
##### 업데이트 성능 (Update Performance)
JSON 객체의 단일 필드를 업데이트하는 성능을 측정합니다.
1. 데이터 크기별: JSON 객체의 단일 필드를 업데이트하는 성능을 측정합니다.
2. 동시성별 (100byte, 1kb): 데이터를 다양한 동시성 수준으로 단일 필드 업데이트 작업을 수행합니다.

#### 4. 결과 확인
본 로컬에서 수행하면 아래와 같은 방식으로 결과가 나옵니다.
```shell
-------------------------------------------------------------
Measure Set Performance - Progress by Size (100 byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Set 1000 operations in 1.3251818s (RPS: 754.613442)
[Total-1][100 byte] Set 1000 operations in 780.6935ms (RPS: 1280.912420)
[Total-1][500 byte] Set 1000 operations in 868.9301ms (RPS: 1150.840557)
[Total-1][1000 byte] Set 1000 operations in 999.8238ms (RPS: 1000.176231)
[Total-1][10000 byte] Set 1000 operations in 1.2781323s (RPS: 782.391619)
[Total-1][100000 byte] Set 1000 operations in 3.047146s (RPS: 328.175939)
-------------------------------------------------------------
Measure Set Performance - 100 byte + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][100 byte] Set 4000 operations in 1.4065185s (RPS: 2843.901449)
[Total-8][100 byte] Set 8000 operations in 1.7671214s (RPS: 4527.136619)
[Total-16][100 byte] Set 16000 operations in 2.4252571s (RPS: 6597.238701)
[Total-32][100 byte] Set 32000 operations in 4.1758783s (RPS: 7663.058571)
-------------------------------------------------------------
Measure Set Performance - 1kb  + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][1000 byte] Set 4000 operations in 1.2187744s (RPS: 3281.985575)
[Total-8][1000 byte] Set 8000 operations in 2.115789s (RPS: 3781.095374)
[Total-16][1000 byte] Set 16000 operations in 2.9169634s (RPS: 5485.156242)
[Total-32][1000 byte] Set 32000 operations in 4.9683356s (RPS: 6440.788742)

-------------------------------------------------------------
Measure Get Performance - increase size (100byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Get 1000 operations in 1.1462733s (RPS: 872.392299)
[Total-1][100 byte] Get 1000 operations in 992.1831ms (RPS: 1007.878485)
[Total-1][1000 byte] Get 1000 operations in 1.315485s (RPS: 760.175905)
[Total-1][10000 byte] Get 1000 operations in 1.211298s (RPS: 825.560680)
[Total-1][100000 byte] Get 1000 operations in 2.7791054s (RPS: 359.828022)
-------------------------------------------------------------
Measure Get Performance - 100 byte + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][100 byte] Get 4000 operations in 1.5121049s (RPS: 2645.319118)
[Total-8][100 byte] Get 8000 operations in 1.6289788s (RPS: 4911.052249)
[Total-16][100 byte] Get 16000 operations in 2.9560816s (RPS: 5412.570478)
[Total-32][100 byte] Get 32000 operations in 4.4068731s (RPS: 7261.384495)
-------------------------------------------------------------
Measure Get Performance - 1kb + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][1000 byte] Get 4000 operations in 1.2542926s (RPS: 3189.048552)
[Total-8][1000 byte] Get 8000 operations in 1.586307s (RPS: 5043.159994)
[Total-16][1000 byte] Get 16000 operations in 2.7451266s (RPS: 5828.510787)
[Total-32][1000 byte] Get 32000 operations in 4.3629055s (RPS: 7334.561796)

-------------------------------------------------------------
Measure Update Performance (updating one field) - Increase size (100byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Update 1000 operations in 1.1755575s (RPS: 850.660219)
[Total-1][100 byte] Update 1000 operations in 1.0224089s (RPS: 978.082253)
[Total-1][500 byte] Update 1000 operations in 954.1732ms (RPS: 1048.027758)
[Total-1][1000 byte] Update 1000 operations in 876.9975ms (RPS: 1140.254106)
[Total-1][10000 byte] Update 1000 operations in 892.0004ms (RPS: 1121.075730)
[Total-1][100000 byte] Update 1000 operations in 918.4566ms (RPS: 1088.783074)
-------------------------------------------------------------
Measure Update Performance (updating one field) - 100 byte + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][100 byte] Update 4000 operations in 1.1158085s (RPS: 3584.844532)
[Total-8][100 byte] Update 8000 operations in 1.5309941s (RPS: 5225.363050)
[Total-16][100 byte] Update 16000 operations in 2.5220799s (RPS: 6343.970308)
[Total-32][100 byte] Update 32000 operations in 4.1471011s (RPS: 7716.233395)
-------------------------------------------------------------
Measure Update Performance (updating one field) - 1kb + increase goroutine count (4 to 8 to 16)
-------------------------------------------------------------
[Total-4][1000 byte] Update 4000 operations in 1.1800931s (RPS: 3389.563078)
[Total-8][1000 byte] Update 8000 operations in 1.5368841s (RPS: 5205.337214)
[Total-16][1000 byte] Update 16000 operations in 2.5278661s (RPS: 6329.449175)
[Total-32][1000 byte] Update 32000 operations in 4.4212289s (RPS: 7237.806665)
```
