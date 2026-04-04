FROM registry.lazycat.cloud/lzc-cli/devshell:v0.0.5

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories

RUN apk add --no-cache bash go nodejs npm \
  && echo "root:root" | chpasswd

CMD ["sleep", "infinity"]
