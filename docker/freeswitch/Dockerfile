# 云枢 FreeSWITCH 镜像 - 包含 mod_audio_stream
# 本镜像基于 bytedesk/freeswitch:latest，并包含了 mod_audio_stream 模块
# 支持通过 WebSocket 实时推送语音流用于 AI 分析

# 基础: 我们刚才提交的镜像 (yunshu/freeswitch:ai-audio) 已经包含所有东西
# 下面只是为了文档说明，实际使用时请直接用 yunshu/freeswitch:ai-audio

FROM yunshu/freeswitch:ai-audio

LABEL maintainer="yunshu"
LABEL description="Yunshu FreeSWITCH with mod_audio_stream for real-time AI audio streaming"
LABEL version="1.0"

# 模块已经预安装在 /usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so
# 配置已经准备完成

# 暴露必要端口
EXPOSE 5060/tcp 5060/udp 5080/tcp 5080/udp
EXPOSE 8021/tcp
EXPOSE 7443/tcp
EXPOSE 16384-32768/udp

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD /usr/local/freeswitch/bin/fs_cli -x 'show channels 2>&1' || exit 1

WORKDIR /
