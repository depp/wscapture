all: wscapture.bundle.js
clean:
	rm -f wscapture.bundle.js
.PHONY: all
wscapture.bundle.js: wscapture.js
	./node_modules/.bin/rollup -c rollup.config.js
