server: templ
	go build -o ./bin/server ./cmd

templ:
	templ generate -path ./views

frontend: $(wildcard ./src/*)
	npx tailwindcss -i src/css/input.css -o src/css/output.css
	rm -rf dist/*
	cp -r src/* dist/.
	cp ./node_modules/@xterm/xterm/css/xterm.css dist/css/include/xterm.css
	cp ./node_modules/@xterm/xterm/lib/xterm.js dist/js/include/xterm.js
	cp ./node_modules/codejar/dist/codejar.js dist/js/include/codejar.js

luadocker: docker/lua/Dockerfile
	docker build -t runlua:latest ./docker/lua/

starter: docker/starter.c
	gcc -g docker/starter.c -o bin/starter

all: luadocker starter templ server frontend

# TODO: figure out how to restrict access to this binary
# install: starter
# 	cp bin/starter /usr/bin/starter
# 	chmod 711 /usr/bin/starter
# 	chmod ug+s /usr/bin/starter
# 	chown /usr/bin/starter root
# 	chgrp /usr/bin/starter root
#
clean:
	rm bin/* static/*
