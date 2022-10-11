![MIT](https://img.shields.io/badge/license-MIT-green)
[![CI](https://github.com/jing332/tts-server-go/actions/workflows/main.yml/badge.svg)](https://github.com/jing332/tts-server-go/actions/workflows/main.yml)
[![Test](https://github.com/jing332/tts-server-go/actions/workflows/test.yml/badge.svg)](https://github.com/jing332/tts-server-go/actions/workflows/test.yml)
![GitHub release (latest by date)](https://img.shields.io/github/downloads/jing332/tts-server-go/latest/total)

# 安卓系统推荐使用 tts-server-android 安装即用
https://github.com/jing332/tts-server-android

# 下载
请从 [Release](https://github.com/jing332/tts-server-go/releases) 下载稳定版。

或者从 [Actions](https://github.com/jing332/tts-server-go/actions) 下载最新代码构建程序。

# 使用方法
直接运行，默认监听1233端口。

然后浏览器 `http://localhost:1233` 打开网页，选好配置导入阅读后即可朗读。

# 提示

接口与[ms-ra-forwarder](https://github.com/wxxxcxx/ms-ra-forwarder) 相同:

微软Azure接口(延迟高): `http://localhost:1233/api/azure`

Edge大声朗读接口: `http://localhost:1233/api/ra`
