# banshee

A prometheus push gateway

需要给接口 POST 如下侧格式的信息，所有 POST 的值，需要写在 body 内传输，不要写表单。

```
curl -XPOST '127.0.0.1:2336/custom_data/' -d '{
    "app":"appname",
    "metric":"api_range",
    "value":"12345",
    "timeout":"60",
    "api":"/",
    "method":"GET",
    "protocol":"HTTP"
}'
```

app、metric、value、timeout 是必备的字段。
除去必须的字段外，可以自定义字段，自定义字段以 kv 键值对的形式提交。

所有 kv 的 value 必须是 string，不能包含数组或 map。
metric 只允许使用大小写和 _ 下划线，相同 metric 的 JSON 结构应该相同。

timeout 为采集过期时间（单位为s）。
例如 timeoue 值为 60 表示如果 60s 内没有新上报的数据，则会删除条 metric 记录。
timeout 最小值为 60。
相同 metric 的 timeout 值应该相同。
如果不需要该功能，则 timeout=""。
