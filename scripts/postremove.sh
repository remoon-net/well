#!/bin/sh
set -e

systemctl disable well-net.service
systemctl stop well-net.service
