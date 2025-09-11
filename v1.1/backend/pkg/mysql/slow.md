1.开启慢查询日志
   首先，检查MySQL的慢查询日志配置：
   -- 查看当前慢查询配置
   SHOW VARIABLES LIKE 'slow_query%';
   SHOW VARIABLES LIKE 'long_query_time';
   SHOW VARIABLES LIKE 'log_queries_not_using_indexes';

-- 开启慢查询日志（需要SUPER权限）
-- 设置日志输出到表和数据
SET GLOBAL log_output = 'TABLE,FILE';
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;  -- 设置超过1秒的查询为慢查询
SET GLOBAL log_queries_not_using_indexes = 'ON';


2. 分析现有表结构
-- 查看各表的数据量
   SELECT
   table_name,
   table_rows,
   ROUND(data_length/1024/1024, 2) AS '数据大小(MB)',
   ROUND(index_length/1024/1024, 2) AS '索引大小(MB)'
   FROM information_schema.tables
   WHERE table_schema = 'canvascloud'
   ORDER BY (data_length + index_length) DESC;
执行这条语句后，你能清楚地看到：
   •哪些表的数据量最大（table_rows最多）。
   •哪些表占用了最多的存储空间（数据大小(MB)+ 索引大小(MB)最大）。
   •每张表的索引开销有多大（比较 索引大小(MB)和 数据大小(MB)）。
-- 查看pictures表结构（通常是大表）
   DESCRIBE pictures;

-- 查看索引情况
    SHOW INDEX FROM pictures;
    SHOW INDEX FROM spaces;
    SHOW INDEX FROM space_users;


3. 查找慢查询
   -- 方法1：使用慢查询日志表（如果已开启），注意是表，不是日志
   SELECT * FROM mysql.slow_log
   ORDER BY start_time DESC
   LIMIT 10;

-- 方法2：查看当前正在执行的慢查询
    SHOW PROCESSLIST;
本质
实时快照：查看当前所有连接和正在执行的请求

-- 方法3：查看最近的慢查询统计
    SELECT * FROM performance_schema.events_statements_summary_by_digest
    ORDER BY SUM_TIMER_WAIT DESC
    LIMIT 1;
这条查询的意思是：从性能摘要表中，按照SQL语句总耗时（SUM_TIMER_WAIT） 从高到低排序，取出最耗时的前10个类型的语句。
4. 分析具体慢查询



5. 使用EXPLAIN分析查询计划


字段名

type

最核心的性能指标，直接反映了MySQL为了找到数据是如何访问表的

目标： 至少达到 range级别。
警戒线： 看到 ALL (全表扫描) 就要高度警惕，通常意味着严重性能问题

key
显示了MySQL实际使用的索引

目标： 显示为一个索引的名称 (如 idx_user_id)。
警戒线： 看到 NULL 表示没有使用任何索引，必须检查原因 

rows
MySQL预估需要扫描多少行记录才能返回结果

目标： 数值越小越好。通常这个值应该远小于表中的总行数。
警戒线： 数值非常大（例如上万、上百万），意味着查询可能很慢。

Extra
提供了关于MySQL如何解析和执行查询的额外重要信息

目标： 看到 Using index(覆盖索引) 是最好的情况之一。
警戒线： 看到 Using filesort (额外排序) 或 Using temporary(使用临时表)，通常意味着需要优化

possible_keys
显示了MySQL能够使用的索引列表

参考： 如果这里列出了索引，但 key却是 NULL，说明MySQL没有使用它们，这可能是因为查询写法导致索引失效，或者优化器认为全表扫描更快。



6. 其他： 
确认MySQL的数据目录（datadir）
    SHOW VARIABLES LIKE 'datadir';
搜索日志文件就行
   "C:\mysql\MySQL Server 8.0\Data\LWHTHINK-slow.log"
使用 SHOW PROFILES(适用于即时测试)
       开启即使测试
SET profiling = 1; 

SELECT SLEEP(1); -- 睡眠 1 毫秒，比你的阈值 0.0001 秒 (0.1毫秒) 要长

1. 验证复合索引 idx_user_status (user_id, status)
   -- 查询特定用户的失败任务
   EXPLAIN SELECT id, name, prompt, status, user_id
   FROM i_tasks
   WHERE user_id = 733617822210461696 AND status = 'failed';
2. 验证复合索引 idx_user_time (user_id, create_time, update_time)
   -- 查询用户近期创建的任务（时间范围查询）
   EXPLAIN SELECT id, name, status, create_time, update_time
   FROM i_tasks
   WHERE user_id = 733617822210461696
   AND create_time > '2025-08-06 00:00:00';