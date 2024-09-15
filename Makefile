
example:
	go build -o example ./examples

test:
	go test -v -cover

benchmark:
	go test -run=^$$ -bench=BenchmarkUDPServerWrite -v

.PHONY: example, test, benchmark