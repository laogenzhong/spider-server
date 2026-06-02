-- https://redis.io/commands/eval/
-- https://stackoverflow.com/questions/29594517/lua-script-in-redis-hmget-with-table
for x = 1, #KEYS do
    local key = KEYS[x]

    -- k v k1 v1
    local keys = {}
    local values = {}
    for i = 1, #ARGV do
        -- key
        if i % 2 == 1 then
            table.insert(keys, ARGV[i])
        else
            table.insert(values, ARGV[i])
        end
    end

    local gets = redis.call("HMGET", key, unpack(keys))

    local updates = {}

    for i = 1, #gets do
        local value = tonumber(gets[i])

        local hkey = keys[i]
        local newVal = tonumber(values[i])

        if value ~= nil and newVal > value then
            table.insert(updates, hkey)
            table.insert(updates, values[i])
        end
    end

    if #updates > 0 then
        redis.call("HSET", key, unpack(updates))
    end

end