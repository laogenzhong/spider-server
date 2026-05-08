upstream gateway_ws_servers {

    hash $uri consistent;

    server gateway-v5-worker-texas-0:9080;

    server gateway-v5-worker-texas-1:9080;

    server gateway-v5-worker-texas-2:9080;

    server gateway-v5-worker-texas-3:9080;

}
通过 Nginx upstream 的一致性哈希 来决定请求转发到哪个 worker。
它会对这个字符串做 hash，然后在这几个节点中选一个：
只要 URI 不变，比如一直是：
那么 hash 结果基本就不变，请求就会一直落到同一个 worker。这个对于 WebSocket 很重要，因为 WS 是长连接，同一个用户或同一个会话最好稳定打到同一个 gateway 节点。


1001:1 -> SessionEntity{Attach: {"_salt": "...", "_ttl": "3600"}}

服务端生成 token -> token 返回给客户端
服务端只保存 salt/ttl/attach -> 不保存 token 原文
客户端后续带 token 请求 -> FromToken 解析 token 得到 uid/scopeID/salt
服务端用 uid/scopeID 查 store -> 比较 store 里的 _salt 和 token 里的 salt
一致则 token 有效，不一致则 token 失效

