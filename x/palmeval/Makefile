TARGETS = palm-exp-0 \
		  palm-exp-1 \
		  palm-exp-2 \
		  palm-exp-3 \
		  palm-exp-4

TERMSHOTS = palm-exp-0.png \
		    palm-exp-1.png \
		    palm-exp-2.png \
		    palm-exp-3.png \
		    palm-exp-4.png

.PHONY: all
all: $(TARGETS) $(TERMSHOTS)

%: %.go
	go build -o $@ $^

%.png: %
	termshot -c -f $@ -- ./$^

.PHONY: clean
clean:
	rm -f $(TARGETS)
