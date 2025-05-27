SHELL = /bin/bash
GO_FILES = $(shell find . -name "*.go" -type f)
TARGETS = palm-exp-0 \
		  palm-exp-1 \
		  palm-exp-2 \
		  palm-exp-3 \
		  palm-exp-4 \
		  palm-exp-5 \
		  palm-exp-6

TERMSHOTS = palm-exp-0.png \
		    palm-exp-1.png \
		    palm-exp-2.png \
		    palm-exp-3.png \
		    palm-exp-4.png \
		    palm-exp-5.png \
		    palm-exp-6.png

palm-montage.png: $(TERMSHOTS)
	montage palm-exp-{0..6}.png -tile 7x1 -geometry +0+0 palm-montage.png

.PHONY: all
all: $(TARGETS) $(TERMSHOTS)

%: %.go
	go build -o $@ $^

%.png: %
	termshot -c -f $@ -- ./$^

.PHONY: clean
clean:
	rm -f $(TARGETS)
	rm -f $(TERMSHOTS)
