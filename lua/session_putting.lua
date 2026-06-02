-- https://redis.io/commands/eval/
-- https://stackoverflow.com/questions/29594517/lua-script-in-redis-hmget-with-table
-- KEYS[1]: hash key
-- KEYS[2..N1]: fields for HMSET
-- KEYS[N1+1..N2]: fields for conditional updates
-- KEYS[N2+1..N3]: fields for deletion
-- ARGV: Values for HMSET (N1) and conditional updates (N2-N1)

local n1 = tonumber(ARGV[1]) -- HMSET 结束位置索引
local n2 = tonumber(ARGV[2]) -- 条件更新结束位置索引

local hash_full_key = "ds:u/-/s/-/:" .. KEYS[1]
local expire_ttl = KEYS[2]

-- HMSET 部分
if n1 > 0 then
    local updates = {}
    for i = 2, n1 + 1 do
        table.insert(updates, KEYS[i])       -- 字段名
        table.insert(updates, ARGV[i])      -- 对应值
    end
    redis.call("HMSET", hash_full_key, unpack(updates))
end

-- 条件更新部分
if n2 > n1 then
    -- 获取当前 hash 中所有待更新字段的值
    local fields_to_update = {}
    for i = n1 + 2, n2 + 1 do
        table.insert(fields_to_update, KEYS[i])  -- 获取需要更新的字段名
    end
    local current_values = redis.call("HMGET", hash_full_key, unpack(fields_to_update))

    -- 准备需要更新的字段和值
    local update_fields = {}
    for i = 1, #fields_to_update do
        local field = fields_to_update[i]
        local new_value = tonumber(ARGV[n1 + 1 + i])
        local current_value = tonumber(current_values[i])

        -- 如果字段不存在或者现有值小于新值，则准备更新
        if current_value == nil or current_value < new_value then
            table.insert(update_fields, field)
            table.insert(update_fields, new_value)
        end
    end

    -- 如果有需要更新的字段，批量执行 HSET
    if #update_fields > 0 then
        redis.call("HSET", hash_full_key, unpack(update_fields))
    end
end

-- HDEL 部分
if #KEYS > n2 + 1 then
    local delete_fields = {}
    for i = n2 + 2, #KEYS do
        table.insert(delete_fields, KEYS[i])
    end
    redis.call("HDEL", hash_full_key, unpack(delete_fields))
end

if tonumber(expire_ttl) > 0 then
    redis.call("EXPIRE", hash_full_key, expire_ttl)
end
