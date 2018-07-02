.PHONY: clean


gtags:
	gogtags

run:
	go run main.go -apiport 3000 -p2pport 4000 -first

run1:
	go run main.go -apiport 3001 -p2pport 4001 -first

build:
	go build -o my-block-chain

clean:
	$(RM) my-block-chain
