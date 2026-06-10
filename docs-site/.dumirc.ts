import { defineConfig } from 'dumi';

export default defineConfig({
  outputPath: 'dist',
  base: '/',
  publicPath: '/',
  themeConfig: {
    name: '云枢声讯',
    logo: '/logo.svg',
    footer: '云枢声讯呼叫中心 © 2026',
    nav: [
      { title: '产品', link: '/product/overview' },
      { title: '指南', link: '/guide/introduction' },
      { title: '架构', link: '/architecture/overview' },
      { title: '话务流程', link: '/telephony/yunshu-phone' },
      { title: '部署', link: '/deployment/local' },
      { title: '运维', link: '/operations/sipp' },
      { title: 'API', link: '/api/cti' },
    ],
    sidebar: {
      '/product': [
        {
          title: '产品',
          children: [
            { title: '产品概览', link: '/product/overview' },
            { title: '功能矩阵', link: '/product/features' },
            { title: '部署模式', link: '/product/deployment-modes' },
            { title: '路线图', link: '/product/roadmap' },
          ],
        },
      ],
      '/guide': [
        {
          title: '指南',
          children: [
            { title: '项目介绍', link: '/guide/introduction' },
            { title: '快速开始', link: '/guide/quick-start' },
            { title: '项目结构', link: '/guide/project-layout' },
          ],
        },
      ],
      '/architecture': [
        {
          title: '架构',
          children: [
            { title: '总体架构', link: '/architecture/overview' },
            { title: '服务边界', link: '/architecture/services' },
            { title: '事件与工作流', link: '/architecture/workflows' },
            { title: '通话记录与计费', link: '/architecture/cdr-billing' },
          ],
        },
      ],
      '/telephony': [
        {
          title: '话务流程',
          children: [
            { title: '云枢声讯', link: '/telephony/yunshu-phone' },
            { title: 'SIP 注册', link: '/telephony/sip-register' },
            { title: '云枢声讯呼出', link: '/telephony/dialpad-outbound' },
            { title: '客户呼入', link: '/telephony/inbound' },
            { title: 'API 外呼', link: '/telephony/api-outbound' },
            { title: '批量外呼', link: '/telephony/batch-outbound' },
            { title: '呼叫状态机', link: '/telephony/state-machine' },
            { title: '呼叫流程详解', link: '/telephony/call-flow-detail' },
          ],
        },
      ],
      '/deployment': [
        {
          title: '部署',
          children: [
            { title: '本地部署', link: '/deployment/local' },
            { title: '生产部署', link: '/deployment/production' },
            { title: 'FreeSWITCH', link: '/deployment/freeswitch' },
            { title: 'Kamailio', link: '/deployment/kamailio' },
            { title: 'RTPEngine', link: '/deployment/rtpengine' },
            { title: '配置说明', link: '/deployment/config' },
            { title: '环境变量', link: '/deployment/env' },
            { title: '数据库表', link: '/deployment/database' },
            { title: '上线检查清单', link: '/deployment/checklist' },
          ],
        },
      ],
      '/operations': [
        {
          title: '运维',
          children: [
            { title: 'SIPp 验证', link: '/operations/sipp' },
            { title: '日志与排障', link: '/operations/troubleshooting' },
            { title: '监控指标', link: '/operations/metrics' },
            { title: 'Runbook', link: '/operations/runbook' },
            { title: '日志字段', link: '/operations/log-fields' },
          ],
        },
      ],
      '/api': [
        {
          title: 'API',
          children: [
            { title: 'CTI API', link: '/api/cti' },
            { title: 'ESL 控制 API', link: '/api/esl' },
            { title: 'WebSocket', link: '/api/websocket' },
            { title: 'Webhook 回调', link: '/api/webhook' },
            { title: '错误码', link: '/api/errors' },
            { title: '第三方接入', link: '/api/integration' },
          ],
        },
      ],
      '/reference': [
        {
          title: '参考',
          children: [
            { title: '术语表', link: '/reference/glossary' },
            { title: 'FAQ', link: '/reference/faq' },
          ],
        },
      ],
    },
  },
});
