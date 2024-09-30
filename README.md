# serve-dir

Portable HTTP directory server inspired by [qrcp](https://github.com/claudiodangelis/qrcp).

## Install

```
go install github.com/vearutop/serve-dir@latest
$(go env GOPATH)/bin/serve-dir --help
```

Or download binary from [releases](https://github.com/vearutop/serve-dir/releases).

### Linux AMD64

```
wget https://github.com/vearutop/serve-dir/releases/latest/download/linux_amd64.tar.gz && tar xf linux_amd64.tar.gz && rm linux_amd64.tar.gz
./serve-dir -version
```

### Linux ARM64

```
wget https://github.com/vearutop/serve-dir/releases/latest/download/linux_arm64.tar.gz && tar xf linux_arm64.tar.gz && rm linux_arm64.tar.gz
./serve-dir -version
```

### Macos Intel

```
wget https://github.com/vearutop/serve-dir/releases/latest/download/darwin_amd64.tar.gz && tar xf darwin_amd64.tar.gz && rm darwin_amd64.tar.gz
codesign -s - ./serve-dir
./serve-dir -version
```

### Macos Apple Silicon (M1, etc...)

```
wget https://github.com/vearutop/serve-dir/releases/latest/download/darwin_arm64.tar.gz && tar xf darwin_arm64.tar.gz && rm darwin_arm64.tar.gz
codesign -s - ./serve-dir
./serve-dir -version
```

## Usage

Run `serve-dir` in the directory you want to serve. HTTP address and QR code will appear.


<img width="236" alt="image" src="https://github.com/user-attachments/assets/b59db572-d96e-4c6d-979e-d9b8bbf54115">

