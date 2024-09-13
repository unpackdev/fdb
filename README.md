# (f)db

Something about this database like fast database or f**k database idk...



## Benchmarks

```
go test -bench=. -v
goos: linux
goarch: amd64
pkg: github.com/unpackdev/fdb
cpu: AMD Ryzen Threadripper 3960X 24-Core Processor 
BenchmarkUDPServerWrite
2024/09/13 18:10:12 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:13 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:14 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:16 UDP Server started on 127.0.0.1:8781
BenchmarkUDPServerWrite-48    	  252528	      4498 ns/op
BenchmarkUDPServerRead
2024/09/13 18:10:18 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:19 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:20 UDP Server started on 127.0.0.1:8781
2024/09/13 18:10:21 UDP Server started on 127.0.0.1:8781
BenchmarkUDPServerRead-48     	   91202	     13363 ns/op
PASS
ok  	github.com/unpackdev/fdb	11.463s
```