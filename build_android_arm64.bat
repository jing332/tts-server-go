SET GOOS=android
SET GOARCH=arm64
SET CGO_ENABLED=1
SET CC=E:\Android_NDK\android-ndk-r24\toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android23-clang.cmd

go build -buildvcs=false -o output/tts-server-go_arm64

pause