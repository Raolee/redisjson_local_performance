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
2. 로컬로 실행했을 때, **`6000 RPS 이상으로 JSON SET을 처리해낸다. 이 정도면 왠만한 비즈니스 요구사항을 만족할 수치로 본다.`**
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
1. Gourtine 수 보다, 실제 PoolSize 가 작다면 요청을 다 처리하지 못하고 대기하여 성능의 결과가 좋지 않게 나옵니다.
2. MinIdleConns 가 너무 작으면, 내부적으로 커넥션을 재사용하지 못해서, 성능의 결과가 조금 더 좋지 않게 나옵니다.
3. MaxIdleConns 은 작으면 성능에 영향을 주지만, PoolSize와 동일하게 설정해도됩니다. 단, Redis server가 입장에서 사용치 않는 socket을 계속 유휴상태로 두게 되므로 정작 필요한 connection을 맺지 못하는 리스크한 상황이 발생할 수 있습니다.
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
Measure Set Performance - Progress by Size
-------------------------------------------------------------
[Total-1][132 byte] Set 1000 operations in 918.6047ms (RPS: 1088.607537)
	Min: 0s Max: 2.0528ms Avg: 918.578µs
	P50: 1ms P95: 1.7083ms P99: 2.0006ms P99.9: 2.0528ms
[Total-1][222 byte] Set 1000 operations in 716.9869ms (RPS: 1394.725622)
	Min: 0s Max: 1.9997ms Avg: 716.986µs
	P50: 999.3µs P95: 1.0048ms P99: 1.0091ms P99.9: 1.9997ms
[Total-1][622 byte] Set 1000 operations in 995.1394ms (RPS: 1004.884341)
	Min: 0s Max: 4.0004ms Avg: 995.139µs
	P50: 1.0001ms P95: 1.9996ms P99: 2.004ms P99.9: 4.0004ms
[Total-1][1122 byte] Set 1000 operations in 1.1124162s (RPS: 898.944118)
	Min: 0s Max: 6.2446ms Avg: 1.112416ms
	P50: 1.0002ms P95: 2.0009ms P99: 2.0155ms P99.9: 6.2446ms
[Total-1][10122 byte] Set 1000 operations in 1.2515489s (RPS: 799.009931)
	Min: 0s Max: 7.2425ms Avg: 1.250547ms
	P50: 1.0008ms P95: 2.0012ms P99: 2.012ms P99.9: 7.2425ms
[Total-1][100122 byte] Set 1000 operations in 2.5292214s (RPS: 395.378594)
	Min: 986.3µs Max: 17.0222ms Avg: 2.523221ms
	P50: 2.0019ms P95: 4.4967ms P99: 10.0108ms P99.9: 17.0222ms
-------------------------------------------------------------
Measure Set Performance - maybe 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][222 byte] Set 4000 operations in 1.1230941s (RPS: 3561.589363)
	Min: 0s Max: 7.466ms Avg: 1.120592ms
	P50: 1.0004ms P95: 2.0009ms P99: 2.0074ms P99.9: 6.9048ms
[Total-8][222 byte] Set 8000 operations in 1.7402512s (RPS: 4597.037485)
	Min: 0s Max: 15.5897ms Avg: 1.733128ms
	P50: 1.9929ms P95: 2.9979ms P99: 4.4051ms P99.9: 15.0318ms
[Total-16][222 byte] Set 16000 operations in 2.5260729s (RPS: 6333.942302)
	Min: 0s Max: 33.1589ms Avg: 2.510669ms
	P50: 2.0055ms P95: 4.0018ms P99: 7.0005ms P99.9: 28.7275ms
[Total-32][222 byte] Set 32000 operations in 3.9744219s (RPS: 8051.485425)
	Min: 0s Max: 28.9832ms Avg: 3.949713ms
	P50: 3.9883ms P95: 6.9983ms P99: 11.4847ms P99.9: 19.0487ms
-------------------------------------------------------------
Measure Set Performance - maybe 1kb  + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1122 byte] Set 4000 operations in 1.2545985s (RPS: 3188.270989)
	Min: 0s Max: 11.2414ms Avg: 1.2516ms
	P50: 1.002ms P95: 2.0028ms P99: 2.0575ms P99.9: 11.0564ms
[Total-8][1122 byte] Set 8000 operations in 1.7528287s (RPS: 4564.051239)
	Min: 0s Max: 17.3252ms Avg: 1.742921ms
	P50: 1.9943ms P95: 2.9995ms P99: 4ms P99.9: 14.1219ms
[Total-16][1122 byte] Set 16000 operations in 2.6976744s (RPS: 5931.034524)
	Min: 835.8µs Max: 42.7741ms Avg: 2.682487ms
	P50: 2.3459ms P95: 4.007ms P99: 7.5479ms P99.9: 29.2409ms
[Total-32][1122 byte] Set 32000 operations in 4.5074439s (RPS: 7099.367338)
	Min: 851.7µs Max: 26.9989ms Avg: 4.479447ms
	P50: 4.0012ms P95: 7.9975ms P99: 12.0724ms P99.9: 24.4364ms
-------------------------------------------------------------
Measure Set Performance - 10kb  + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10122 byte] Set 4000 operations in 2.2275732s (RPS: 1795.676120)
	Min: 982.2µs Max: 27.6202ms Avg: 2.218064ms
	P50: 2.0001ms P95: 3.9994ms P99: 8.6988ms P99.9: 22.6195ms
[Total-8][10122 byte] Set 8000 operations in 3.6312042s (RPS: 2203.125894)
	Min: 898.6µs Max: 40.8316ms Avg: 3.61133ms
	P50: 3.0022ms P95: 6.0172ms P99: 14.9948ms P99.9: 25.3705ms
[Total-16][10122 byte] Set 16000 operations in 6.6359339s (RPS: 2411.115035)
	Min: 875.2µs Max: 27.3672ms Avg: 6.60311ms
	P50: 5.9998ms P95: 13.0019ms P99: 18.9897ms P99.9: 24.7582ms
[Total-32][10122 byte] Set 32000 operations in 12.0458154s (RPS: 2656.524190)
	Min: 992.5µs Max: 59.4632ms Avg: 11.982267ms
	P50: 11.0018ms P95: 22.1234ms P99: 30.0001ms P99.9: 48.9894ms

-------------------------------------------------------------
Measure Get Performance - process by size
-------------------------------------------------------------
[Total-1][10 byte] Get 1000 operations in 852.0021ms (RPS: 1173.706027)
	Min: 0s Max: 3.0002ms Avg: 852.002µs
	P50: 999.6µs P95: 1.0062ms P99: 2.0003ms P99.9: 3.0002ms
[Total-1][100 byte] Get 1000 operations in 907.5836ms (RPS: 1101.826873)
	Min: 0s Max: 4.9995ms Avg: 907.583µs
	P50: 999.7µs P95: 1.0511ms P99: 2.0025ms P99.9: 4.9995ms
[Total-1][500 byte] Get 1000 operations in 962.4243ms (RPS: 1039.042759)
	Min: 0s Max: 5.9434ms Avg: 962.424µs
	P50: 999.9µs P95: 1.9912ms P99: 2.0036ms P99.9: 5.9434ms
[Total-1][1000 byte] Get 1000 operations in 927.5299ms (RPS: 1078.132360)
	Min: 0s Max: 2.0077ms Avg: 926.531µs
	P50: 999.9µs P95: 1.0092ms P99: 2.0011ms P99.9: 2.0077ms
[Total-1][10000 byte] Get 1000 operations in 1.2827759s (RPS: 779.559391)
	Min: 0s Max: 6.9988ms Avg: 1.282775ms
	P50: 1.0047ms P95: 2.0086ms P99: 4.2586ms P99.9: 6.9988ms
[Total-1][100000 byte] Get 1000 operations in 2.6929762s (RPS: 371.336368)
	Min: 997.6µs Max: 12.0015ms Avg: 2.691975ms
	P50: 2.7149ms P95: 3.9938ms P99: 7.6454ms P99.9: 12.0015ms
-------------------------------------------------------------
Measure Get Performance - maybe 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][100 byte] Get 4000 operations in 1.4206433s (RPS: 2815.625851)
	Min: 0s Max: 15.2555ms Avg: 1.419393ms
	P50: 1.0042ms P95: 2.0061ms P99: 3.0008ms P99.9: 12.9941ms
[Total-8][100 byte] Get 8000 operations in 1.827014s (RPS: 4378.729446)
	Min: 0s Max: 18.9982ms Avg: 1.81564ms
	P50: 1.9974ms P95: 2.9996ms P99: 4.003ms P99.9: 11.1344ms
[Total-16][100 byte] Get 16000 operations in 3.0478362s (RPS: 5249.625948)
	Min: 701.4µs Max: 23.3403ms Avg: 3.032326ms
	P50: 2.9958ms P95: 5.3967ms P99: 8.6109ms P99.9: 15.7836ms
[Total-32][100 byte] Get 32000 operations in 4.6706205s (RPS: 6851.338061)
	Min: 988.3µs Max: 36.9069ms Avg: 4.633187ms
	P50: 4.0058ms P95: 8.0013ms P99: 13.0103ms P99.9: 29.7423ms
-------------------------------------------------------------
Measure Get Performance - maybe 1kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1000 byte] Get 4000 operations in 1.2405158s (RPS: 3224.465178)
	Min: 0s Max: 5.9987ms Avg: 1.239513ms
	P50: 1.0012ms P95: 2.002ms P99: 2.0176ms P99.9: 4.9906ms
[Total-8][1000 byte] Get 8000 operations in 1.7011325s (RPS: 4702.749492)
	Min: 0s Max: 42.6696ms Avg: 1.686192ms
	P50: 1.9894ms P95: 2.7025ms P99: 3.9975ms P99.9: 40.9868ms
[Total-16][1000 byte] Get 16000 operations in 2.5487414s (RPS: 6277.608234)
	Min: 573.4µs Max: 11.5331ms Avg: 2.527475ms
	P50: 2.0077ms P95: 4.0037ms P99: 5.9944ms P99.9: 10.5266ms
[Total-32][1000 byte] Get 32000 operations in 4.5533609s (RPS: 7027.775901)
	Min: 986µs Max: 22.3239ms Avg: 4.507815ms
	P50: 4.004ms P95: 7.9998ms P99: 11.0024ms P99.9: 15.6642ms
-------------------------------------------------------------
Measure Get Performance - maybe 10kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10000 byte] Get 4000 operations in 1.5613741s (RPS: 2561.846005)
	Min: 0s Max: 7.6286ms Avg: 1.557996ms
	P50: 1.6463ms P95: 2.0107ms P99: 3.0119ms P99.9: 6.7808ms
[Total-8][10000 byte] Get 8000 operations in 2.3343035s (RPS: 3427.146470)
	Min: 523.8µs Max: 26.8505ms Avg: 2.321318ms
	P50: 2.0021ms P95: 3.9972ms P99: 6.0087ms P99.9: 16.401ms
[Total-16][10000 byte] Get 16000 operations in 4.2286819s (RPS: 3783.684935)
	Min: 993.9µs Max: 32.0838ms Avg: 4.205635ms
	P50: 3.9969ms P95: 7.9321ms P99: 12.0451ms P99.9: 19.655ms
[Total-32][10000 byte] Get 32000 operations in 7.9328939s (RPS: 4033.836883)
	Min: 1.0066ms Max: 41.1736ms Avg: 7.837346ms
	P50: 7.0011ms P95: 14.079ms P99: 21.6644ms P99.9: 33.7125ms

-------------------------------------------------------------
Measure Update Performance (updating one field) - progress by size
-------------------------------------------------------------
[Total-1][1 byte] Update 1000 operations in 1.1279729s (RPS: 886.546122)
	Min: 0s Max: 21.0056ms Avg: 1.127972ms
	P50: 1.0001ms P95: 2.0018ms P99: 5.0036ms P99.9: 21.0056ms
[Total-1][10 byte] Update 1000 operations in 881.0288ms (RPS: 1135.036675)
	Min: 0s Max: 4.9957ms Avg: 881.028µs
	P50: 999.7µs P95: 1.0198ms P99: 2.0012ms P99.9: 4.9957ms
[Total-1][50 byte] Update 1000 operations in 963.094ms (RPS: 1038.320247)
	Min: 0s Max: 3.9999ms Avg: 962.094µs
	P50: 999.9µs P95: 1.9987ms P99: 2.0023ms P99.9: 3.9999ms
[Total-1][100 byte] Update 1000 operations in 1.0079996s (RPS: 992.063886)
	Min: 0s Max: 5.2511ms Avg: 1.007001ms
	P50: 1.0001ms P95: 1.9944ms P99: 2.003ms P99.9: 5.2511ms
[Total-1][1000 byte] Update 1000 operations in 875.1615ms (RPS: 1142.646243)
	Min: 0s Max: 3.996ms Avg: 874.619µs
	P50: 999.7µs P95: 1.0822ms P99: 1.9998ms P99.9: 3.996ms
[Total-1][10000 byte] Update 1000 operations in 1.2286578s (RPS: 813.896270)
	Min: 0s Max: 9.9508ms Avg: 1.22266ms
	P50: 1.0013ms P95: 2.0007ms P99: 3.0059ms P99.9: 9.9508ms
-------------------------------------------------------------
Measure Update Performance (updating one field) - maybe 100 byte + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][10 byte] Update 4000 operations in 1.2644259s (RPS: 3163.491036)
	Min: 0s Max: 5.0116ms Avg: 1.262613ms
	P50: 1.0012ms P95: 2.0039ms P99: 2.0137ms P99.9: 5.0116ms
[Total-8][10 byte] Update 8000 operations in 1.5848371s (RPS: 5047.837409)
	Min: 0s Max: 8.9665ms Avg: 1.57309ms
	P50: 1.917ms P95: 2.0111ms P99: 3.0031ms P99.9: 7.4783ms
[Total-16][10 byte] Update 16000 operations in 2.6452312s (RPS: 6048.620627)
	Min: 0s Max: 13.0102ms Avg: 2.616167ms
	P50: 2.0146ms P95: 4.1655ms P99: 6.9852ms P99.9: 10.7319ms
[Total-32][10 byte] Update 32000 operations in 4.4316475s (RPS: 7220.790914)
	Min: 518.6µs Max: 24.5204ms Avg: 4.397206ms
	P50: 4.0002ms P95: 7.9998ms P99: 12ms P99.9: 19.7316ms
-------------------------------------------------------------
Measure Update Performance (updating one field) - maybe 1kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][100 byte] Update 4000 operations in 1.2356822s (RPS: 3237.078271)
	Min: 0s Max: 10.0045ms Avg: 1.232183ms
	P50: 1.0016ms P95: 2.0019ms P99: 2.0155ms P99.9: 9.0041ms
[Total-8][100 byte] Update 8000 operations in 1.6274246s (RPS: 4915.742333)
	Min: 0s Max: 9.8101ms Avg: 1.618798ms
	P50: 1.9912ms P95: 2.3835ms P99: 3.0119ms P99.9: 9.585ms
[Total-16][100 byte] Update 16000 operations in 2.7046094s (RPS: 5915.826515)
	Min: 0s Max: 16.2799ms Avg: 2.687483ms
	P50: 2.4939ms P95: 4.62ms P99: 6.9945ms P99.9: 15.0342ms
[Total-32][100 byte] Update 32000 operations in 4.1612439s (RPS: 7690.008269)
	Min: 0s Max: 48.9536ms Avg: 4.124896ms
	P50: 3.9977ms P95: 6.9983ms P99: 9.3276ms P99.9: 43.4296ms
-------------------------------------------------------------
Measure Update Performance (updating one field) - maybe 10kb + increase goroutine count (4 to 32)
-------------------------------------------------------------
[Total-4][1000 byte] Update 4000 operations in 1.3880267s (RPS: 2881.788945)
	Min: 0s Max: 5.9904ms Avg: 1.384027ms
	P50: 1.0074ms P95: 2.005ms P99: 2.3321ms P99.9: 5.9904ms
[Total-8][1000 byte] Update 8000 operations in 1.7585883s (RPS: 4549.103392)
	Min: 0s Max: 16.0012ms Avg: 1.752715ms
	P50: 1.996ms P95: 2.9987ms P99: 4.0038ms P99.9: 15.0026ms
[Total-16][1000 byte] Update 16000 operations in 2.7415791s (RPS: 5836.052660)
	Min: 526.4µs Max: 17.5897ms Avg: 2.722332ms
	P50: 2.3108ms P95: 4.9961ms P99: 7.9993ms P99.9: 14.0029ms
[Total-32][1000 byte] Update 32000 operations in 4.5126182s (RPS: 7091.226995)
	Min: 986.1µs Max: 41.009ms Avg: 4.486274ms
	P50: 4.0015ms P95: 7.996ms P99: 12.7286ms P99.9: 35.0102ms
```
