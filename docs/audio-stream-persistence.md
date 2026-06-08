# FreeSWITCH mod_audio_stream 配置持久化指南

## 概述

本文档说明如何持久化 mod_audio_stream 的配置，确保容器重启后配置不会丢失。

## 当前配置

### 1. 模块自动加载配置

**文件**: `/usr/local/freeswitch/conf/autoload_configs/modules.conf.xml`

```xml
<load module="mod_audio_stream" />
```

### 2. 模块参数配置

**文件**: `/usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml`

```xml
<configuration name="audio_stream.conf" description="Audio Stream Module Configuration">
  <settings>
    <param name="debug" value="true"/>
  </settings>
</configuration>
```

## 持久化方案

### 方案 1: Docker Volume (推荐)

#### 步骤 1: 创建配置目录

```bash
mkdir -p ./docker/freeswitch/conf/autoload_configs
```

#### 步骤 2: 复制配置文件

```bash
# 从运行中的容器复制配置
docker cp cc-freeswitch:/usr/local/freeswitch/conf/autoload_configs/modules.conf.xml ./docker/freeswitch/conf/autoload_configs/
docker cp cc-freeswitch:/usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml ./docker/freeswitch/conf/autoload_configs/
```

#### 步骤 3: 修改 docker-compose.yml

```yaml
freeswitch:
  image: yunshu/freeswitch:latest
  container_name: cc-freeswitch
  restart: always
  ports:
    - "5080:5080/udp"
    - "8021:8021"
  volumes:
    - ./docker/freeswitch/conf:/usr/local/freeswitch/conf:ro
    - freeswitch_log:/usr/local/freeswitch/log
    - freeswitch_recordings:/var/freeswitch/recordings
  environment:
    - ESL_PASSWORD=ClueCon
  networks:
    - callcenter_net
```

#### 步骤 4: 重启容器

```bash
docker-compose down freeswitch
docker-compose up -d freeswitch
```

### 方案 2: Dockerfile (推荐用于生产环境)

#### 创建 Dockerfile

```dockerfile
FROM yunshu/freeswitch:latest

# 复制自定义配置
COPY docker/freeswitch/conf/autoload_configs/modules.conf.xml /usr/local/freeswitch/conf/autoload_configs/
COPY docker/freeswitch/conf/autoload_configs/audio_stream.conf.xml /usr/local/freeswitch/conf/autoload_configs/

# 其他自定义配置
# COPY docker/freeswitch/conf/sip_profiles/ /usr/local/freeswitch/conf/sip_profiles/
# COPY docker/freeswitch/conf/dialplan/ /usr/local/freeswitch/conf/dialplan/

EXPOSE 5080/udp 8021

CMD ["freeswitch", "-nonat", "-rp"]
```

#### 构建镜像

```bash
docker build -t yunshu/freeswitch:audio-stream .
docker push yunshu/freeswitch:audio-stream
```

#### 更新 docker-compose.yml

```yaml
freeswitch:
  image: yunshu/freeswitch:audio-stream  # 使用自定义镜像
  container_name: cc-freeswitch
  restart: always
  # ... 其他配置保持不变
```

### 方案 3: 初始化脚本 (开发环境)

#### 创建初始化脚本

```bash
mkdir -p ./docker/freeswitch/scripts
```

```bash
#!/bin/bash
# docker/freeswitch/scripts/init-audio-stream.sh

set -e

echo "初始化 mod_audio_stream 配置..."

# 检查模块配置是否存在
if ! grep -q 'mod_audio_stream' /usr/local/freeswitch/conf/autoload_configs/modules.conf.xml; then
    echo "添加 mod_audio_stream 到 modules.conf.xml..."
    sed -i '/<load module="mod_commands" \/>/a\    <load module="mod_audio_stream" \/>' \
        /usr/local/freeswitch/conf/autoload_configs/modules.conf.xml
fi

# 创建音频流配置文件
if [ ! -f /usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml ]; then
    echo "创建 audio_stream.conf.xml..."
    cat > /usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml << 'EOF'
<configuration name="audio_stream.conf" description="Audio Stream Module Configuration">
  <settings>
    <param name="debug" value="true"/>
  </settings>
</configuration>
EOF
fi

echo "mod_audio_stream 配置完成"
```

#### 修改 Dockerfile

```dockerfile
FROM yunshu/freeswitch:latest

# 复制初始化脚本
COPY docker/freeswitch/scripts/init-audio-stream.sh /docker-entrypoint-initdb.d/
RUN chmod +x /docker-entrypoint-initdb.d/init-audio-stream.sh

EXPOSE 5080/udp 8021

CMD ["freeswitch", "-nonat", "-rp"]
```

## 配置文件说明

### modules.conf.xml

```xml
<!-- 确保包含 mod_audio_stream 的加载指令 -->
<configuration name="modules" description="FreeSWITCH Modules">
  <modules>
    <!-- ... 其他模块 ... -->
    <load module="mod_commands" />
    <load module="mod_audio_stream" />  <!-- 添加这行 -->
    <load module="mod_conference" />
    <!-- ... 其他模块 ... -->
  </modules>
</configuration>
```

### audio_stream.conf.xml

```xml
<configuration name="audio_stream.conf" description="Audio Stream Module Configuration">
  <settings>
    <!-- 调试模式：生产环境建议设置为 false -->
    <param name="debug" value="true"/>

    <!-- 可选：自定义设置 -->
    <!-- <param name="buffer-size" value="2048"/> -->
    <!-- <param name="sample-rate" value="16000"/> -->
  </settings>
</configuration>
```

## 验证配置

### 1. 检查配置文件

```bash
# 检查 modules.conf.xml
docker exec cc-freeswitch grep mod_audio_stream /usr/local/freeswitch/conf/autoload_configs/modules.conf.xml

# 检查 audio_stream.conf.xml
docker exec cc-freeswitch cat /usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml
```

### 2. 运行验证脚本

```bash
./scripts/verify-audio-stream.sh
```

### 3. 测试容器重启

```bash
# 重启容器
docker-compose restart freeswitch

# 等待容器启动
sleep 10

# 验证模块仍然加载
docker exec cc-freeswitch fs_cli -x "show modules" | grep audio_stream
```

## 故障排查

### 问题: 配置未生效

**症状**: 模块未自动加载

**解决方案**:

1. 检查配置文件是否正确挂载
2. 验证配置文件语法
3. 手动加载模块测试
4. 查看 FreeSWITCH 启动日志

```bash
# 查看启动日志
docker logs cc-freeswitch --tail 100 | grep -E "(mod_audio_stream|modules)"

# 手动加载模块
docker exec cc-freeswitch fs_cli -x "load mod_audio_stream"

# 检查错误信息
docker exec cc-freeswitch fs_cli -x "load mod_audio_stream"
```

### 问题: 配置文件被覆盖

**症状**: 自定义配置在容器重启后丢失

**解决方案**:

1. 使用 volume 挂载配置文件（方案 1）
2. 使用自定义 Dockerfile（方案 2）
3. 不要直接修改容器内文件

## 最佳实践

1. **版本控制**: 将配置文件放入 Git 管理
2. **环境隔离**: 开发、测试、生产使用不同配置
3. **配置验证**: 使用 CI/CD 验证配置语法
4. **文档记录**: 记录所有配置变更
5. **备份恢复**: 定期备份配置文件

## 相关资源

- [mod_audio_stream 官方文档](https://freeswitch.org/confluence/display/FREESWITCH/mod_audio_stream)
- [FreeSWITCH 模块配置](https://freeswitch.org/confluence/display/FREESWITCH/Mod_commands)
- [Docker 数据持久化](https://docs.docker.com/storage/volumes/)
