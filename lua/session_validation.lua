local uid_key = KEYS[1]
local once_value = KEYS[2]

local replay_ttl_seconds = 120

-- seconds
--local current_time = tonumber(KEYS[3])
--local max_count = tonumber(KEYS[4])
--local window_ttl_seconds = 120
-- 窗口保持 120s
--local window_start = current_time - window_ttl_seconds

local hash_full_key = "ds:u/-/s/-/:" .. uid_key
--local window_key = hash_full_key .. ":window"
local replay_key = "ds_once:u/-/s/-/:" .. uid_key .. ":" .. once_value

-- fixme 限流先delay了
--local request_count = redis.call("ZCARD", window_key) -- 获取当前请求数
--if request_count > 0 then
--    -- 如果超过限制，返回拒绝状态
--    if request_count >= max_count then
--        -- 使用 Sorted Set 来存储请求时间戳，限流操作
--        redis.call("ZREMRANGEBYSCORE", window_key, 0, window_start) -- 删除过期请求
--        return { 1 }
--    end
--
--    -- 使用 Sorted Set 来存储请求时间戳，限流操作
--    redis.call("ZREMRANGEBYSCORE", window_key, 0, window_start) -- 删除过期请求
--end
--
--local once_exists = redis.call("ZADD", window_key, "NX", current_time, once_value)
--if once_exists == 0 then
--    return { 2 }
--end
--
--redis.call("EXPIRE", window_key, window_ttl_seconds)

-- 到这意 token 已校 所可直 check 是存该值
-- redis verison > 7.0.0
local window_ok = redis.call("SET", replay_key, 1, "NX", "GET", "EX", replay_ttl_seconds)
if window_ok == '1' then
    return { "1" }
end

-- 第二步：查询哈希中的所有值
local ttl_field = "_ttl"
local hash_field_values = redis.call("HMGET", hash_full_key, ttl_field, unpack(ARGV))
local ttl_value = table.remove(hash_field_values, 1)
-- ttl 是必须存在的值 所以这里 可以用来判断 key 是否存在
if ttl_value == false then
    return { "2" }
end

redis.call("EXPIRE", hash_full_key, ttl_value)

return { "0", unpack(hash_field_values) }
