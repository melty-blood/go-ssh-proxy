
# go-ssh-proxy

### 代号 `Honoka`

> 1. 工具可以达到 `ssh -NL 20022:IP_ADDR:22 -J TypeMoon satsuki@SERVER_IP -p 8606` 这样的效果
> 2. 具有网络检测功能, 可以查看是否能到达某个ip和端口
> 3. 附加功能: 查看本地某张图片在其他目录是否也存在, 就是查重(这个是个人需要)

------


```go

# linux
# -ldflags '-extldflags "-static"' 静态编译可以解决动态链接库依赖问题
CGO_ENABLED=0 GOARCH=amd64 go build -o ./honoka_proxy -a -ldflags '-extldflags "-static"' honoka.go

# windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ./honoka_proxy.exe -a -ldflags '-extldflags "-static"' honoka.go

```


### LoveLive 镇楼
![./lovelive.jpg](./lovelive.jpg)

