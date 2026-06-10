# 云枢声讯文档站

本目录是云枢声讯文档站，基于 [dumi](https://d.umijs.org/) 构建。

## 本地开发

```bash
cd docs-site
npm install
npm run dev
```

## 构建

```bash
npm run build
```

或使用部署脚本：

```bash
./deploy/build.sh
```

构建产物：

```text
docs-site/dist
```

可部署到 Nginx、Vercel、Netlify、GitHub Pages、对象存储静态网站等。

## Nginx 部署示例

```bash
npm run build
sudo mkdir -p /var/www/yunshu-docs
sudo rsync -av --delete dist/ /var/www/yunshu-docs/
sudo cp deploy/nginx.conf /etc/nginx/conf.d/yunshu-docs.conf
sudo nginx -t
sudo systemctl reload nginx
```

将 `deploy/nginx.conf` 中的 `server_name` 替换为你的文档域名。

## 目录

```text
docs/
  guide/          项目介绍和快速开始
  architecture/   架构和工作流
  telephony/      呼入、呼出、API 外呼、批量外呼
  deployment/     本地和生产部署
  operations/     SIPp 验证和排障
  api/            API 文档
  reference/      FAQ 和术语表
```
