#!/bin/bash
# FreeSWITCH mod_audio_stream 验证脚本
# 用于验证音频流模块是否正确加载和工作

set -e

echo "=========================================="
echo "FreeSWITCH mod_audio_stream 验证工具"
echo "=========================================="
echo ""

# 容器名称
CONTAINER_NAME="cc-freeswitch"

# 1. 检查容器是否运行
echo "[1/6] 检查 FreeSWITCH 容器状态..."
if ! docker ps --filter "name=$CONTAINER_NAME" --filter "status=running" | grep -q "$CONTAINER_NAME"; then
    echo "❌ FreeSWITCH 容器未运行，请先启动容器"
    exit 1
fi
echo "✅ FreeSWITCH 容器运行中"

# 2. 检查 mod_audio_stream 模块是否加载
echo ""
echo "[2/6] 检查 mod_audio_stream 模块加载状态..."
MODULE_CHECK=$(docker exec "$CONTAINER_NAME" fs_cli -x "show modules" 2>/dev/null | grep -i "mod_audio_stream" || echo "")

if echo "$MODULE_CHECK" | grep -q "mod_audio_stream"; then
    echo "✅ mod_audio_stream 模块已加载"
    echo "   模块信息: $(echo "$MODULE_CHECK" | head -n 1)"
else
    echo "⚠️  mod_audio_stream 模块未加载，尝试手动加载..."
    LOAD_RESULT=$(docker exec "$CONTAINER_NAME" fs_cli -x "load mod_audio_stream" 2>/dev/null)

    if echo "$LOAD_RESULT" | grep -q "+OK"; then
        echo "✅ mod_audio_stream 模块加载成功"
    else
        echo "❌ mod_audio_stream 模块加载失败"
        echo "   错误: $LOAD_RESULT"
        exit 1
    fi
fi

# 3. 检查 uuid_audio_stream 命令是否可用
echo ""
echo "[3/6] 检查 uuid_audio_stream 命令可用性..."
CMD_CHECK=$(docker exec "$CONTAINER_NAME" fs_cli -x "uuid_audio_stream" 2>/dev/null)

if echo "$CMD_CHECK" | grep -q "USAGE"; then
    echo "✅ uuid_audio_stream 命令可用"
    echo "   用法: $CMD_CHECK"
else
    echo "❌ uuid_audio_stream 命令不可用"
    exit 1
fi

# 4. 检查模块文件是否存在
echo ""
echo "[4/6] 检查 mod_audio_stream.so 模块文件..."
MODULE_FILE=$(docker exec "$CONTAINER_NAME" test -f /usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so && echo "存在" || echo "不存在")

if [ "$MODULE_FILE" = "存在" ]; then
    FILE_INFO=$(docker exec "$CONTAINER_NAME" ls -lh /usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so 2>/dev/null | awk '{print $5, $9}')
    echo "✅ 模块文件存在: $FILE_INFO"
else
    echo "❌ 模块文件不存在"
    exit 1
fi

# 5. 检查配置文件
echo ""
echo "[5/6] 检查 audio_stream.conf.xml 配置文件..."
CONFIG_FILE=$(docker exec "$CONTAINER_NAME" test -f /usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml && echo "存在" || echo "不存在")

if [ "$CONFIG_FILE" = "存在" ]; then
    echo "✅ 配置文件存在"
    docker exec "$CONTAINER_NAME" cat /usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml 2>/dev/null | grep -A 3 "settings"
else
    echo "⚠️  配置文件不存在，但模块仍可工作"
fi

# 6. 检查模块在 modules.conf.xml 中的自动加载配置
echo ""
echo "[6/6] 检查模块自动加载配置..."
AUTO_LOAD=$(docker exec "$CONTAINER_NAME" grep -i "mod_audio_stream" /usr/local/freeswitch/conf/autoload_configs/modules.conf.xml 2>/dev/null || echo "")

if echo "$AUTO_LOAD" | grep -q "load module=\"mod_audio_stream\""; then
    echo "✅ 模块已配置为自动加载"
else
    echo "⚠️  模块未配置为自动加载，建议添加以下配置到 modules.conf.xml:"
    echo '   <load module="mod_audio_stream" />'
fi

echo ""
echo "=========================================="
echo "验证完成！"
echo "=========================================="
echo ""
echo "下一步操作："
echo "1. 创建测试呼叫: originate user/1000 &park()"
echo "2. 启动音频流: uuid_audio_stream <uuid> start wss://your-server/audio mono 16k"
echo "3. 停止音频流: uuid_audio_stream <uuid> stop"
echo ""
echo "或使用 Go 代码中的 AIVoiceEngine 来自动管理音频流"
