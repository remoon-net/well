# Changelog

## [0.0.8] - 2025-02-09

- 优化: 现在在未修改初始密码的情况下可以直接进入管理页面了

## [0.0.7] - 2025-02-08

- 修复: 修改WireGuard配置监听事件由`OnRecordUpdateRequest`改为`OnRecordUpdate`, 因为有些事件不是通过请求触发的

## [0.0.6] - 2025-02-08

- 修复: 数据正确性检查应当放在 OnRecordValidate 中
- 添加: 支持事件: `onRecordValidate`
- 优化: hookjs 不包装 GoError

## [0.0.5] - 2025-02-08

- 添加: hookjs 支持事件 `onRecordCreate`, `onRecordUpdate`, `onRecordDelete`

## [0.0.4] - 2025-02-08

- 优化: 除了手机端, 默认都开机自启
- 优化: 现在都支持使用 name 进行排序
- 添加: 支持 hookjs, 这样允许任何人连接时不再需要外挂脚本了

## [0.0.3] - 2025-02-03

- 修复: 通过分享链接导入的节点默认不设置 ipv4, 因为有很多服务监听在IPv4内网上

## [0.0.2] - 2025-02-02

- 更改: IPv6 地址段改为 2001:00f0::/28. 因为 ULA 的 AAAA 记录会被 openwrt 过滤掉
- 优化: IPv6 地址现在也支持 auto 了

## [0.0.1] - 2025-01-31

- 完成了节点导入
