# Altun-core build entry points.
#   make core   build the Go binaries of this module into bin/
#   make bft    build the BFT engines (HotStuff fork, MirBFT) via bft/build.sh
#   make proxy  build the switching proxy tools (proxy/) into bin/
#   make all    all of the above
#   make test   unit tests of this module
#   make proto  regenerate proto/altunpb from proto/consensus.proto

.PHONY: all core bft proxy test proto clean

all: core bft proxy

core:
	bash bin/build.sh

bft:
	bash bft/build.sh

proxy:
	cd altunsm && go build ./...
	cd proxy && go build -o ../bin/ ./cmd/...

test:
	go test ./...

proto:
	bash scripts/gen_proto.sh

clean:
	rm -f bin/cft-server bin/cft-client bin/cft-cmd bin/cft-kv bin/forensic-verify \
	      bin/cft-adapter bin/bft-adapter bin/altun-controller bin/altunwan-probe \
	      bin/bft-server bin/bft-client bin/bft-window bin/hskv \
	      bin/altun-proxy bin/altun-ctl bin/altun-load bin/altun-check
	rm -rf mir/bin
