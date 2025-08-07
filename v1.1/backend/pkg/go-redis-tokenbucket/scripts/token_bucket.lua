--[[ 脚本参数说明 ]]--
-- KEYS:
--   [1] bucket_key: 令牌桶的Redis键名
--
-- ARGV:
--   [1] token_gen_rate: 生成一个令牌需要的时间（纳秒）
--   [2] bucket_capacity: 桶容量（最大令牌数）
--   [3] token_request: 请求的令牌数量
--   [4] key_expiration: Key过期时间（毫秒）

local bucket_key = KEYS[1] -- 令牌桶标识

-- 从参数中解析数值型参数
local token_gen_rate_ns = tonumber(ARGV[1])
local bucket_capacity = tonumber(ARGV[2])
local token_request = tonumber(ARGV[3])
local key_expiration_ms = tonumber(ARGV[4])
-- 获取当前时间（从Go代码传入）
local current_time_ns = tonumber(ARGV[5])

--------------------------------------------------------------------------------
-- 获取并初始化最后刷新时间
-- 策略：若桶不存在，创建新桶（时间为当前时间）
--------------------------------------------------------------------------------
local last_refresh_time_ns = redis.call('GET', bucket_key)

-- 检查桶是否存在
local is_bucket_new = false
if not last_refresh_time_ns then
    -- 桶不存在：初始化新桶，设置初始时间使桶满
    last_refresh_time_ns = current_time_ns - (bucket_capacity * token_gen_rate_ns)
    is_bucket_new = true
else
    -- 桶存在：转换为数字
    last_refresh_time_ns = tonumber(last_refresh_time_ns)
end

--------------------------------------------------------------------------------
-- 令牌生成计算
-- 公式：生成令牌数 = min(时间差/令牌生成速率, 桶容量)
--------------------------------------------------------------------------------
-- 计算从上一次更新到现在的时间差（纳秒）
local elapsed_ns_since_refresh = current_time_ns - last_refresh_time_ns

-- 确保时间差非负（处理时钟漂移）
elapsed_ns_since_refresh = math.max(elapsed_ns_since_refresh, 0)

-- 计算期间生成的令牌数
local tokens_generated = math.floor(elapsed_ns_since_refresh / token_gen_rate_ns)

-- 应用桶容量限制（不能超过容量上限）
local tokens_available = math.min(tokens_generated, bucket_capacity)

--------------------------------------------------------------------------------
-- 请求处理逻辑
-- 核心：当可用令牌 >= 请求令牌时允许请求
--------------------------------------------------------------------------------
local request_allowed = 0 -- 默认拒绝请求

if tokens_available >= token_request then
    -- 满足请求条件：
    request_allowed = 1

    -- 计算新的刷新时间：
    -- 推进时间 = 原时间 + (请求令牌数 * 令牌生成时间)
    local new_refresh_time_ns = last_refresh_time_ns + (token_request * token_gen_rate_ns)

    -- 更新Redis中的时间戳
    redis.call('SET', bucket_key, new_refresh_time_ns, 'PX', key_expiration_ms)

    -- 移除重复的PEXPIRE调用，SET命令已包含过期时间设置
    -- if is_bucket_new then
    --     redis.call('PEXPIRE', bucket_key, key_expiration_ms)
    -- end
end

--------------------------------------------------------------------------------
-- 返回结果
-- 1 = 允许请求, 0 = 拒绝请求
--------------------------------------------------------------------------------
return request_allowed