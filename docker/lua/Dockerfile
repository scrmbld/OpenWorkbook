FROM nickblah/luajit:2-lua52compat-luarocks-alpine
WORKDIR .
COPY . .
VOLUME /luasource
CMD ["luajit", "/luasource/main.lua"]
