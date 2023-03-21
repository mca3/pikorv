all: pikorv

pikorv: $(shell find . -name "*.go" -type f)
	rm -f pikorv
	go build .
	doas setcap cap_net_admin=+ep pikorv

.PHONY: clean

clean:
	rm -f pikorv
