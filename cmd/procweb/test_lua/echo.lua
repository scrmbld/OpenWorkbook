io.stdout:setvbuf("no")
io.stderr:setvbuf("no")

while true do
	local x = io.stdin:read()
	if x == nil then break end
	print(x)
end
