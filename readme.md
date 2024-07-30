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

- **PoolSize**: 최대 socket connection 수
- **MinIdleConns**: 최소 유휴(idle) connection 수
- **MaxIdleConns**: 최대 유휴(idle) connection 수

##### 위 옵션과의 성능 시나리와의 관계
1. 실제 PoolSize 가 작다면, 아무리 요청을 보내도 커넥션이 부족하여 goroutine이 많아지는 성능의 결과가 좋지 않게 나옵니다.
2. MinIdleConns 가 너무 작으면, 내부적으로 커넥션을 재사용하지 못해서 조금 더 좋지 않은 성능을 보입니다.
3. MaxIdleConns 은 PoolSize와 같은 값을 설정해도 되지면, Redis server가 여러 Client 를 받는 상황을 고려하면 redis의 자원을 과도하게 사용하게 됩니다.
```go
// Connect to the Redis server
rdb := redis.NewClient(&redis.Options{
    Addr:         fmt.Sprintf("%s:%d", container.Host, container.DefaultPort()),
    PoolSize:     64, // socket connection을 맺는 최대 수
    MinIdleConns: 16, // 최소 유휴 connection 수
    MaxIdleConns: 32, // 최대 유휴 connection 수
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
따뜻해져라...
따뜻해졌지?

-------------------------------------------------------------
Measure Set Performance - Progress by Size (100 byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Set 1000 operations in 922.3445ms (RPS: 1084.193596)
[Total-1][100 byte] Set 1000 operations in 855.8548ms (RPS: 1168.422494)
[Total-1][500 byte] Set 1000 operations in 880.0066ms (RPS: 1136.355114)
[Total-1][1000 byte] Set 1000 operations in 816.3636ms (RPS: 1224.944375)
[Total-1][10000 byte] Set 1000 operations in 1.1175261s (RPS: 894.833687)
[Total-1][100000 byte] Set 1000 operations in 2.4602087s (RPS: 406.469581)
-------------------------------------------------------------
Measure Set Performance - 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][100 byte] Set 4000 operations in 1.1615907s (RPS: 3443.553741)
[Total-8][100 byte] Set 8000 operations in 1.8072508s (RPS: 4426.613063)
[Total-16][100 byte] Set 16000 operations in 2.4774761s (RPS: 6458.185409)
[Total-32][100 byte] Set 32000 operations in 4.2843884s (RPS: 7468.977369)
-------------------------------------------------------------
Measure Set Performance - 1kb  + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1000 byte] Set 4000 operations in 1.2024537s (RPS: 3326.531408)
[Total-8][1000 byte] Set 8000 operations in 1.823888s (RPS: 4386.234242)
[Total-16][1000 byte] Set 16000 operations in 2.5506944s (RPS: 6272.801634)
[Total-32][1000 byte] Set 32000 operations in 5.5629542s (RPS: 5752.339288)
-------------------------------------------------------------
Measure Set Performance - 10kb  + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10000 byte] Set 4000 operations in 2.2309953s (RPS: 1792.921751)
[Total-8][10000 byte] Set 8000 operations in 3.4327764s (RPS: 2330.475122)
[Total-16][10000 byte] Set 16000 operations in 6.6393908s (RPS: 2409.859652)
[Total-32][10000 byte] Set 32000 operations in 11.8301922s (RPS: 2704.943374)

-------------------------------------------------------------
Measure Get Performance - increase size (100byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Get 1000 operations in 863.5776ms (RPS: 1157.973528)
[Total-1][100 byte] Get 1000 operations in 863.5647ms (RPS: 1157.990826)
[Total-1][500 byte] Get 1000 operations in 937.1186ms (RPS: 1067.100792)
[Total-1][1000 byte] Get 1000 operations in 851.9983ms (RPS: 1173.711262)
[Total-1][10000 byte] Get 1000 operations in 1.2886438s (RPS: 776.009631)
[Total-1][100000 byte] Get 1000 operations in 2.5290199s (RPS: 395.410095)
-------------------------------------------------------------
Measure Get Performance - 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][100 byte] Get 4000 operations in 1.3021409s (RPS: 3071.864189)
[Total-8][100 byte] Get 8000 operations in 1.7026513s (RPS: 4698.554543)
[Total-16][100 byte] Get 16000 operations in 2.5101339s (RPS: 6374.161952)
[Total-32][100 byte] Get 32000 operations in 4.5651414s (RPS: 7009.640490)
-------------------------------------------------------------
Measure Get Performance - 1kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1000 byte] Get 4000 operations in 1.3018015s (RPS: 3072.665072)
[Total-8][1000 byte] Get 8000 operations in 1.7169591s (RPS: 4659.400448)
[Total-16][1000 byte] Get 16000 operations in 2.5439351s (RPS: 6289.468627)
[Total-32][1000 byte] Get 32000 operations in 4.3897454s (RPS: 7289.716620)
-------------------------------------------------------------
Measure Get Performance - 10kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10000 byte] Get 4000 operations in 1.5782829s (RPS: 2534.399885)
[Total-8][10000 byte] Get 8000 operations in 2.247821s (RPS: 3559.002252)
[Total-16][10000 byte] Get 16000 operations in 4.2545394s (RPS: 3760.689112)
[Total-32][10000 byte] Get 32000 operations in 7.5655268s (RPS: 4229.712067)

-------------------------------------------------------------
Measure Update Performance (updating one field) - Increase size (100byte to 100kb)
-------------------------------------------------------------
[Total-1][10 byte] Update 1000 operations in 830.0006ms (RPS: 1204.818406)
[Total-1][100 byte] Update 1000 operations in 1.0500831s (RPS: 952.305584)
[Total-1][500 byte] Update 1000 operations in 881.0001ms (RPS: 1135.073651)
[Total-1][1000 byte] Update 1000 operations in 915.9984ms (RPS: 1091.704964)
[Total-1][10000 byte] Update 1000 operations in 1.0281462s (RPS: 972.624321)
[Total-1][100000 byte] Update 1000 operations in 977.5427ms (RPS: 1022.973216)
-------------------------------------------------------------
Measure Update Performance (updating one field) - 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][100 byte] Update 4000 operations in 1.153446s (RPS: 3467.869324)
[Total-8][100 byte] Update 8000 operations in 1.573465s (RPS: 5084.320274)
[Total-16][100 byte] Update 16000 operations in 2.4578575s (RPS: 6509.734596)
[Total-32][100 byte] Update 32000 operations in 4.2704108s (RPS: 7493.424286)
-------------------------------------------------------------
Measure Update Performance (updating one field) - 1kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1000 byte] Update 4000 operations in 1.2919139s (RPS: 3096.181564)
[Total-8][1000 byte] Update 8000 operations in 1.5984548s (RPS: 5004.833418)
[Total-16][1000 byte] Update 16000 operations in 2.4583829s (RPS: 6508.343350)
[Total-32][1000 byte] Update 32000 operations in 4.260945s (RPS: 7510.071123)
-------------------------------------------------------------
Measure Update Performance (updating one field) - 10kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10000 byte] Update 4000 operations in 1.3952209s (RPS: 2866.929531)
[Total-8][10000 byte] Update 8000 operations in 1.7446088s (RPS: 4585.555226)
[Total-16][10000 byte] Update 16000 operations in 2.4422384s (RPS: 6551.366975)
[Total-32][10000 byte] Update 32000 operations in 4.1825854s (RPS: 7650.770263)
```
