server: templ frontend
	go build -o ./bin/server ./cmd

templ:
	templ generate -path ./views

frontend:
	npm --prefix frontend run build

luadocker: docker/lua/Dockerfile
	docker build -t runlua:latest ./docker/lua/

starter: docker/starter.c
	gcc -g docker/starter.c -o bin/starter

all: luadocker starter templ server

# TODO: figure out how to restrict access to this binary
# install: starter
# 	cp bin/starter /usr/bin/starter
# 	chmod 711 /usr/bin/starter
# 	chmod ug+s /usr/bin/starter
# 	chown /usr/bin/starter root
# 	chgrp /usr/bin/starter root
