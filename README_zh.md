[English](./README.md) | 中文

---

# well net

well-net 是一款帮助用户和他人建立**内部交流网络**的开源组网软件

## download

https://salt.remoon.cn/

## 内网聊天软件 delta chat

我发现使用邮件协议交流更适合这种随时离线的场景. 所以选择了 [delta chat](https://delta.chat/), 它支持 `remoon@[2001:ff::1]` 这样基于IP的邮箱格式可以完美适配

但目前的开源邮件服务器都默认需要TLS, 而在 well-net 网络中是不需要TLS的, 所以还是需要另外构建一个邮件服务器来适配这个内网交流场景.
可我目前没有时间去做这个, 如果有好用的可以在issues中推荐

推荐的邮件服务器实现需要满足以下几点

1. 支持基于IP格式的邮箱
2. 将`From`邮箱域名改成内网IP格式, 比如 `remoon@remoon.net` 改成 `remoon@[2001:ff::1]`, 统一身份
3. 支持重发. 因为其他节点随时可能离线
4. 也许还有一些其它的?

# 使用演示

## 用户使用演示

https://youtu.be/H-iywrYNtmY

## 服务提供者使用演示

https://youtu.be/D2iu9xNmfR8

# 前端界面

折腾来折腾去还是用 webui 快

[well-webui](https://github.com/remoon-net/well-webui)

# Todo

- [x] 插件机制. `_hookjs` 不算好使, 但可以实现"允许任何人连接"的功能
- [ ] <del>自己所属设备统一管理</del> 暂时不做, 感觉用外部脚本实现也挺简单的
- [ ] 防火墙
- [ ] 支持 socks proxy

# 双重许可

如果你希望作品可以闭源发布, 可以向我购买闭源许可. (基于此贡献代码需要签署 CLA 允许我商业化售卖闭源许可)

当然如果作品是开源的, 只要遵守 GPL3.0 许可即可, 将你的代码开放给软件使用者. GPL3.0 不会传染到服务端, 你可以随意魔改服务端
