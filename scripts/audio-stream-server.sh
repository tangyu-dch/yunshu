#!/bin/bash
# 简单的 WebSocket 音频流测试服务器
# 用于测试 FreeSWITCH 的 mod_audio_stream 功能

set -e

PORT=${1:-8080}
echo "启动简单的 WebSocket 音频流测试服务器..."
echo "监听端口: $PORT"
echo "按 Ctrl+C 停止"
echo ""

# 使用 netcat 或 socat 创建一个简单的 WebSocket 服务器
# 注意：这是一个简单的演示，实际生产环境应该使用真正的 WebSocket 服务器

# 方法1: 使用 Python (如果可用)
if command -v python3 &> /dev/null; then
    python3 -c "
import asyncio
import websockets
import os
import sys

async def audio_handler(websocket, path):
    print(f'客户端连接: {websocket.remote_address}')
    try:
        async for audio_data in websocket:
            # 记录接收到的音频数据
            print(f'收到音频数据: {len(audio_data)} 字节')
            # 可以在这里处理音频数据
    except websockets.exceptions.ConnectionClosed:
        print('客户端断开连接')
    except Exception as e:
        print(f'错误: {e}')

async def main():
    async with websockets.serve(audio_handler, '0.0.0.0', $PORT):
        print(f'WebSocket 服务器运行在 ws://0.0.0.0:$PORT')
        await asyncio.Future()  # 运行永久

try:
    asyncio.run(main())
except KeyboardInterrupt:
    print('服务器停止')
" 2>&1
else
    # 方法2: 使用 Node.js (如果可用)
    if command -v node &> /dev/null; then
        node -e "
const WebSocket = require('ws');
const wss = new WebSocket.Server({ port: $PORT });

wss.on('connection', (ws) => {
    console.log('客户端连接');

    ws.on('message', (message) => {
        console.log('收到音频数据:', message.length, '字节');
    });

    ws.on('close', () => {
        console.log('客户端断开连接');
    });
});

console.log('WebSocket 服务器运行在 ws://0.0.0.0:$PORT');
" 2>&1
    else
        echo "❌ 需要 Python3 或 Node.js 来运行 WebSocket 服务器"
        echo ""
        echo "安装选项："
        echo "  macOS: brew install python3 或 brew install node"
        echo "  Ubuntu/Debian: apt-get install python3 或 apt-get install nodejs"
        echo ""
        echo "或者手动测试："
        echo "  1. 启动 FreeSWITCH 呼叫"
        echo "  2. 使用 fs_cli 执行: uuid_audio_stream <uuid> start wss://your-server/audio mono 16k"
        echo "  3. 检查 FreeSWITCH 日志以确认连接尝试"
        exit 1
    fi
fi
