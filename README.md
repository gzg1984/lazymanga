# 懒鸟软件
- 早起的鸟儿有虫吃
- 懒鸟没虫吃
- Early Bird Has bugs
- Lazy Birdd has no Bug.

# 开发
## 启动后端
- 启动一个终端,执行下面命令
```
lzc-cli project devshell
# 进入容器后
cd backend
go run .
```

## 启动前端

```
lzc-cli project devshell
# 进入容器后
cd ui
npm i
npm run dev
```

## 构建

```
# mac
lzc-cli project build -o release.lpk
# win
lzc-cli.cmd project build -o release.lpk
```

会在当前的目录下构建出一个 lpk 包。

## 安装

```
lzc-cli app install release.lpk
```

会安装在你的微服应用中,安装成功后可在懒猫微服启动器中查看!

## 交流和帮助

你可以在 https://bbs.lazycat.cloud/ 畅所欲言。


# 编译出的lazyiso后台文件应该储存的位置：
/lzcapp/run/mnt/home

#  发布流程
- 将devshell 的 /lzcapp/cache/devshell/backend 目录中编译出的lazyiso文件，通过移动到网盘目录/lzcapp/document/中
```
cd /lzcapp/cache/devshell/backend
go build
mv lazyiso /lzcapp/document/
```
- 通过懒猫网盘将lazyiso 下载到本地 ，然后将lazyiso 文件，拷贝到 dist/ 中
```
mv ~/Downloads/lazyiso dist/
chmod +x dist/lazyiso     
```
- 在本地shell环境中执行 build （windows不可用）  
```
lzc-cli project build  
```
- 本地测试安装
```
lzc-cli app install cloud.lazybird.app.lazyiso-v0.1.0.lpk
```
- 测试通过，则 lzc-cli appstore publish cloud.lazybird.app.lazyiso-v0.1.0.lpk             