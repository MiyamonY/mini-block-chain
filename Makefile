.PHONY: clean

gtags:
	gogtags

build:
	go build -o my-block-chain

clean:
	$(RM) my-block-chain
