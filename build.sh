for os in darwin freebsd windows
do
	GOOS=${os} GOARCH=386 go build -o release/socks2http_${os}_i386
	GOOS=${os} GOARCH=amd64 go build -o release/socks2http_${os}_amd64
done
for arch in 386 amd64 arm arm64
do
	GOOS=linux GOARCH=${arch} go build -o release/socks2http_linux_${arch}
done


