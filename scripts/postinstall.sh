#!/bin/sh
set -e

# 重新加载 systemd 配置
systemctl daemon-reload

# 自动启用并启动服务
systemctl enable well-net.service
systemctl start well-net.service
