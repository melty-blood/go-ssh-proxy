
# go-ssh-proxy

### 代号 `Honoka`

> 1. 工具可以达到 `ssh -NL 20022:IP_ADDR:22 -J TypeMoon satsuki@SERVER_IP -p 8606` 这样的效果
> 2. 具有网络检测功能, 可以查看是否能到达某个ip和端口
> 3. 附加功能: 查看本地某张图片在其他目录是否也存在, 就是查重

------


##### > TIP: 需要注意目前gocv这个扩展在我的电脑(Kubuntu 23.1 Kernel 6.5.0-17, KDE 5.110.0/Plasma 5.27.8)无法正常安装, 所以不建议使用.

```go

# linux
# -ldflags '-extldflags "-static"' 静态编译可以解决动态链接库依赖问题
CGO_ENABLED=0 GOARCH=amd64 go build -o ./honoka_proxy -a -ldflags '-extldflags "-static"' honoka.go

# windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ./honoka_proxy.exe -a -ldflags '-extldflags "-static"' honoka.go

```


### LoveLive 镇楼
![https://github.com/melty-blood/go-ssh-proxy/blob/master/lovelive.jpg](./lovelive.jpg)

